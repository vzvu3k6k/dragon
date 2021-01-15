package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

func init() {
	flag.StringVar(&token, "t", "", "Bot Token")
	flag.Parse()
}

var token string
var buffer = make([][]byte, 0)
var dragoon = Dragoon{}

func main() {
	if token == "" {
		fmt.Printf("No token provided. Please run: %s -t <bot token>", filepath.Base(os.Args[0]))
		return
	}

	err := loadSound()
	if err != nil {
		fmt.Println("Error loading sound: ", err)
		return
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session: ", err)
		return
	}

	dg.AddHandler(messageCreate)
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates)

	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

type Dragoon struct {
	VoiceConnection *discordgo.VoiceConnection
	Timer           *time.Timer
}

func (d Dragoon) SetTimer(s *discordgo.Session, guildID, voiceChannelID string) {
	d.StopTimer()
	d.Timer = time.AfterFunc(5*time.Minute, func() {
		playSound(s, guildID, voiceChannelID)
	})
}

func (d Dragoon) StopTimer() {
	if d.Timer != nil {
		d.Timer.Stop()
	}
}

func (d Dragoon) Exit() {
	if d.VoiceConnection != nil {
		d.VoiceConnection.Disconnect()
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if strings.HasPrefix(m.Content, "!dra-stop") {
		dragoon.StopTimer()
		dragoon.Exit()
		return
	}

	if strings.HasPrefix(m.Content, "!dra-start") {
		g, err := s.State.Guild(m.GuildID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Guildが取得できませんでした :cry:")
			return
		}

		voiceChannelID, err := findTargetVoiceChannelID(g, m.Author.ID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "ボイスチャンネルが取得できませんでした :cry:")
			return
		}

		s.ChannelMessageSend(m.ChannelID, "タイマーをセットしました :dragon_face:")
		dragoon.SetTimer(s, g.ID, voiceChannelID)
	}
}

// Look for the message sender in that guild's current voice states.
func findTargetVoiceChannelID(g *discordgo.Guild, userID string) (string, error) {
	for _, vs := range g.VoiceStates {
		if vs.UserID == userID {
			return vs.ChannelID, nil
		}
	}
	return "", errors.New("Can't find a target voice channel ID")
}

// loadSound attempts to load an encoded sound file from disk.
func loadSound() error {

	file, err := os.Open("airhorn.dca")
	if err != nil {
		fmt.Println("Error opening dca file :", err)
		return err
	}

	var opuslen int16

	for {
		// Read opus frame length from dca file.
		err = binary.Read(file, binary.LittleEndian, &opuslen)

		// If this is the end of the file, just return.
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			err := file.Close()
			if err != nil {
				return err
			}
			return nil
		}

		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			return err
		}

		// Read encoded pcm from dca file.
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			return err
		}

		// Append encoded pcm data to the buffer.
		buffer = append(buffer, InBuf)
	}
}

// playSound plays the current buffer to the provided channel.
func playSound(s *discordgo.Session, guildID, channelID string) (err error) {

	// Join the provided voice channel.
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return err
	}
	dragoon.VoiceConnection = vc

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(250 * time.Millisecond)

	// Start speaking.
	vc.Speaking(true)

	// Send the buffer data.
	for _, buff := range buffer {
		vc.OpusSend <- buff
	}

	// Stop speaking
	vc.Speaking(false)

	// Sleep for a specificed amount of time before ending.
	time.Sleep(250 * time.Millisecond)

	return nil
}
