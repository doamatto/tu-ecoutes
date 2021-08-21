package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
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

	// Set needed intents
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates

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

func fetchVC(s *discordgo.Session, m *discordgo.MessageCreate) string {
	g := m.GuildID
	guild, err := s.State.Guild(g)
	if err != nil {
		log.Panicf("DISCORD: %v", err)
		return ""
	}
	for _, vs := range guild.VoiceStates {
		if vs.UserID == m.Author.ID {
			return vs.ChannelID
		}
	}
	return ""
}

func cmd(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

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
		url := strings.Split(m.Content, " ")
		if len(url) < 2 {
			s.ChannelMessageSend(m.ChannelID, "You need to mention the URL for a video or song (tip: copy and paste the link from your browser, instead of manually typing it in)")
			return
		}
		videoURL := url[1]

		// Establish needed globals
		g := m.GuildID

		// Fetch VC
		vChannel := fetchVC(s, m)
		if vChannel == "" {
			s.ChannelMessageSend(m.ChannelID, "You need to be in a voice channel for the bot to play a song.")
			return
		}

		// Connect over WebRTC
		vc, err := s.ChannelVoiceJoin(g, vChannel, false, false)
		if err != nil {
			log.Panicf("DISCORD: %v", err)
			return
		}

		// Fetch stream through YTDL
		client := youtube.Client{}
		video, err := client.GetVideo(videoURL)
		if err != nil {
			log.Panicf("YTDL: %v", err)
			return
		}
		format := video.Formats.FindByItag(140) // 128kbps M4A
		stream, _, err := client.GetStream(video, format)
		fmt.Println(stream)
		if err != nil {
			log.Panicf("YTDL: %v", err)
			return
		}

		// Load stream to buffer
		var buffer = make([][]byte, 0)
		var opuslen int16

		for {
			err = binary.Read(stream, binary.LittleEndian, &opuslen)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				err := stream.Close()
				if err != nil {
					log.Panicf("BUFFER: %v", err)
					return
				}
				return
			}
			if err != nil {
				log.Panicf("BUFFER: %v", err)
				return
			}

			InBuf := make([]byte, opuslen)
			err = binary.Read(stream, binary.LittleEndian, &InBuf)
			if err != nil {
				log.Panicf("BUFFER: %v", err)
			}
			buffer = append(buffer, InBuf)
		}

		// Play stream in Discord
		// TODO: convert it to a DCA stream before streaming to Discord
		vc.Speaking(true)
		for _, buff := range buffer {
			vc.OpusSend <- buff
		}
		vc.Speaking(false)
		vc.Disconnect()
	}
}
