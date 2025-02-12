package bot

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/sirupsen/logrus"

	"github.com/kubeshop/botkube/pkg/config"
)

// mmChannelType to find Mattermost channel type
type mmChannelType string

const (
	mmChannelPrivate mmChannelType = "P"
	mmChannelPublic  mmChannelType = "O"
)

const (
	// WebSocketProtocol stores protocol initials for web socket
	WebSocketProtocol = "ws://"
	// WebSocketSecureProtocol stores protocol initials for web socket
	WebSocketSecureProtocol = "wss://"
)

// TODO:
// 	- Use latest Mattermost API v6
// 	- Remove usage of `log.Fatal` - return error instead

// MMBot listens for user's message, execute commands and sends back the response
type MMBot struct {
	log             logrus.FieldLogger
	executorFactory ExecutorFactory
	reporter        AnalyticsReporter

	Token            string
	BotName          string
	TeamName         string
	ChannelName      string
	ClusterName      string
	AllowKubectl     bool
	RestrictAccess   bool
	ServerURL        string
	WebSocketURL     string
	WSClient         *model.WebSocketClient
	APIClient        *model.Client4
	DefaultNamespace string
}

// mattermostMessage contains message details to execute command and send back the result
type mattermostMessage struct {
	log             logrus.FieldLogger
	executorFactory ExecutorFactory

	Event         *model.WebSocketEvent
	Response      string
	Request       string
	IsAuthChannel bool
	APIClient     *model.Client4
}

// NewMattermostBot returns new Bot object
func NewMattermostBot(log logrus.FieldLogger, c *config.Config, executorFactory ExecutorFactory, reporter AnalyticsReporter) *MMBot {
	mattermost := c.Communications.GetFirst().Mattermost
	return &MMBot{
		log:              log,
		executorFactory:  executorFactory,
		reporter:         reporter,
		ServerURL:        mattermost.URL,
		BotName:          mattermost.BotName,
		Token:            mattermost.Token,
		TeamName:         mattermost.Team,
		ChannelName:      mattermost.Channels.GetFirst().Name,
		ClusterName:      c.Settings.ClusterName,
		AllowKubectl:     c.Executors.GetFirst().Kubectl.Enabled,
		RestrictAccess:   c.Executors.GetFirst().Kubectl.RestrictAccess,
		DefaultNamespace: c.Executors.GetFirst().Kubectl.DefaultNamespace,
	}
}

// Start establishes mattermost connection and listens for messages
func (b *MMBot) Start(ctx context.Context) error {
	b.log.Info("Starting bot")
	b.APIClient = model.NewAPIv4Client(b.ServerURL)
	b.APIClient.SetOAuthToken(b.Token)

	// Check if Mattermost URL is valid
	checkURL, err := url.Parse(b.ServerURL)
	if err != nil {
		return fmt.Errorf("while parsing Mattermost URL %q: %w", b.ServerURL, err)
	}

	// Create WebSocketClient and handle messages
	b.WebSocketURL = WebSocketProtocol + checkURL.Host
	if checkURL.Scheme == "https" {
		b.WebSocketURL = WebSocketSecureProtocol + checkURL.Host
	}

	// Check connection to Mattermost server
	err = b.checkServerConnection()
	if err != nil {
		return fmt.Errorf("while pinging Mattermost server %q: %w", b.ServerURL, err)
	}

	err = b.reporter.ReportBotEnabled(b.IntegrationName())
	if err != nil {
		return fmt.Errorf("while reporting analytics: %w", err)
	}

	// It is observed that Mattermost server closes connections unexpectedly after some time.
	// For now, we are adding retry logic to reconnect to the server
	// https://github.com/kubeshop/botkube/issues/201
	b.log.Info("BotKube connected to Mattermost!")
	for {
		select {
		case <-ctx.Done():
			b.log.Info("Shutdown requested. Finishing...")
			return nil
		default:
			var appErr *model.AppError
			b.WSClient, appErr = model.NewWebSocketClient4(b.WebSocketURL, b.APIClient.AuthToken)
			if appErr != nil {
				return fmt.Errorf("while creating WebSocket connection: %w", appErr)
			}
			b.listen(ctx)
		}
	}
}

// IntegrationName describes the notifier integration name.
func (b *MMBot) IntegrationName() config.CommPlatformIntegration {
	return config.MattermostCommPlatformIntegration
}

// TODO: refactor - handle and send methods should be defined on Bot level

// Check incoming message and take action
func (mm *mattermostMessage) handleMessage(b MMBot) {
	post := model.PostFromJson(strings.NewReader(mm.Event.Data["post"].(string)))
	channelType := mmChannelType(mm.Event.Data["channel_type"].(string))
	if channelType == mmChannelPrivate || channelType == mmChannelPublic {
		// Message posted in a channel
		// Serve only if starts with mention
		if !strings.HasPrefix(strings.ToLower(post.Message), fmt.Sprintf("@%s ", strings.ToLower(b.BotName))) {
			return
		}
	}

	// Check if message posted in authenticated channel
	if mm.Event.Broadcast.ChannelId == b.getChannel().Id {
		mm.IsAuthChannel = true
	}
	mm.log.Debugf("Received mattermost event: %+v", mm.Event.Data)

	// remove @BotKube prefix if exists
	r := regexp.MustCompile(`^(?i)@BotKube `)
	mm.Request = r.ReplaceAllString(post.Message, ``)

	e := mm.executorFactory.NewDefault(b.IntegrationName(), mm.IsAuthChannel, mm.Request)
	mm.Response = e.Execute()
	mm.sendMessage()
}

// Send messages to Mattermost
func (mm mattermostMessage) sendMessage() {
	mm.log.Debugf("Mattermost incoming Request: %s", mm.Request)
	mm.log.Debugf("Mattermost Response: %s", mm.Response)
	post := &model.Post{}
	post.ChannelId = mm.Event.Broadcast.ChannelId

	if len(mm.Response) == 0 {
		mm.log.Infof("Invalid request. Dumping the response. Request: %s", mm.Request)
		return
	}
	// Create file if message is too large
	if len(mm.Response) >= 3990 {
		res, resp := mm.APIClient.UploadFileAsRequestBody([]byte(mm.Response), mm.Event.Broadcast.ChannelId, mm.Request)
		if resp.Error != nil {
			mm.log.Error("Error occurred while uploading file. Error: ", resp.Error)
		}
		post.FileIds = []string{res.FileInfos[0].Id}
	} else {
		post.Message = formatCodeBlock(mm.Response)
	}

	// Create a post in the Channel
	if _, resp := mm.APIClient.CreatePost(post); resp.Error != nil {
		mm.log.Error("Failed to send message. Error: ", resp.Error)
	}
}

// Check if Mattermost server is reachable
func (b MMBot) checkServerConnection() error {
	// Check api connection
	if _, resp := b.APIClient.GetOldClientConfig(""); resp.Error != nil {
		return resp.Error
	}

	// Get channel list
	_, resp := b.APIClient.GetTeamByName(b.TeamName, "")
	if resp.Error != nil {
		return resp.Error
	}
	return nil
}

// Check if team exists in Mattermost
func (b MMBot) getTeam() *model.Team {
	botTeam, resp := b.APIClient.GetTeamByName(b.TeamName, "")
	if resp.Error != nil {
		b.log.Fatalf("There was a problem finding Mattermost team %s. %s", b.TeamName, resp.Error)
	}
	return botTeam
}

// Check if BotKube user exists in Mattermost
func (b MMBot) getUser() *model.User {
	users, resp := b.APIClient.AutocompleteUsersInTeam(b.getTeam().Id, b.BotName, 1, "")
	if resp.Error != nil {
		b.log.Fatalf("There was a problem finding Mattermost user %s. %s", b.BotName, resp.Error)
	}
	return users.Users[0]
}

// Create channel if not present and add BotKube user in channel
func (b MMBot) getChannel() *model.Channel {
	// Checking if channel exists
	botChannel, resp := b.APIClient.GetChannelByName(b.ChannelName, b.getTeam().Id, "")
	if resp.Error != nil {
		b.log.Fatalf("There was a problem finding Mattermost channel %s. %s", b.ChannelName, resp.Error)
	}

	// Adding BotKube user to channel
	b.APIClient.AddChannelMember(botChannel.Id, b.getUser().Id)
	return botChannel
}

func (b MMBot) listen(ctx context.Context) {
	b.WSClient.Listen()
	defer b.WSClient.Close()
	for {
		select {
		case <-ctx.Done():
			b.log.Info("Shutdown requested. Finishing...")
			return
		case event, ok := <-b.WSClient.EventChannel:
			if !ok {
				if b.WSClient.ListenError != nil {
					b.log.Debugf("while listening on websocket connection: %s", b.WSClient.ListenError.Error())
				}

				b.log.Info("Incoming events channel closed. Finishing...")
				return
			}

			if event == nil {
				b.log.Info("Nil event, ignoring")
				continue
			}

			if event.EventType() != model.WEBSOCKET_EVENT_POSTED {
				// ignore
				continue
			}

			post := model.PostFromJson(strings.NewReader(event.GetData()["post"].(string)))

			// Skip if message posted by BotKube or doesn't start with mention
			if post.UserId == b.getUser().Id {
				continue
			}
			mm := mattermostMessage{
				log:             b.log,
				executorFactory: b.executorFactory,
				Event:           event,
				IsAuthChannel:   false,
				APIClient:       b.APIClient,
			}
			mm.handleMessage(b)
		}
	}
}
