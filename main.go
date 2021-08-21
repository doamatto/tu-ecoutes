package main

import (
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
	"github.com/kkdai/youtube/v2"
)

func main() {
	var err error
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatalf("Missing Discord authentication token. Check README on how to resolve this issue.")
	}
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Error authenticating with Discord's servers. More information to follow: %v", err)
	}

	// Open connection to Discord
	err = s.Open()
	if err != nil {
		log.Fatalf("Cannot connect to Discord's servers. More information to follow: %v", err)
	}
	// Log OK and set status
	log.Println("=== === ===")
	log.Println("Bot is currently running.")
	log.Println("=== === ===")
	s.UpdateGameStatus(0, "Use e.help")

	s.AddHandler(cmd)

	// Gracefully close the Discord session, where possible
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-stop
	s.Close()
	log.Println("Shutting down bot gracefully...")
}

func cmd(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Establish needed globals
	g := m.GuildID
	c := m.ChannelID

	if strings.HasPrefix(m.Content, "e.about") {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:       "About this bot",
			Color:       16724804,
			Description: "This was a bot written by [doamatto](https://www.doamatto.xyz) to both experiment with dca and make a music bot per the request of a person in one of my servers.",
		})
	}
	if strings.HasPrefix(m.Content, "e.h") || strings.HasPrefix(m.Content, "e.help") {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title: "Commands",
			Color: 16724804,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "v.about", Value: "What does this bot do and other FAQs", Inline: false},
				{Name: "v.play", Value: "Play any song that [YouTube DL](http://ytdl-org.github.io/youtube-dl/supportedsites.html) gives access to", Inline: false},
			},
		})
	}
	if strings.HasPrefix(m.Content, "e.play") {
		// Check for arguments

		// Fetch stream through YTDL
		client := youtube.Client{}
		video, err := client.GetVideo(videoURL)
		if err != nil {
			log.Panicf("%v", err)
		}
		format := video.Formats.FindByItag(140) // 128kbps M4A
		stream, err := client.GetStreamURL(video, format)
		fmt.Println(stream)
		if err != nil {
			log.Panicf("%v", err)
		}

		// Convert to DCA
		options := dca.StdEncodeOptions
		options.RawOutput = true
		options.Bitrate = 64
		options.Application = "lowdelay"
		encodeSession, err := dca.EncodeFile(stream, options)
		if err != nil {
			log.Panicf("%v", err)
		}
		defer encodeSession.Cleanup()

		// Start playback to Discord
		done := make(chan error)
		dca.NewStream(encodeSession, vc, done)
		er := <-done
		if er != nil && er != io.EOF {
			log.Panicf("DCA: %v", er)
		}
	}
}
