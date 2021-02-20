package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/diamondburned/arikawa/v2/api"
	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/diamondburned/arikawa/v2/gateway"
	"github.com/diamondburned/arikawa/v2/session"
	"github.com/diamondburned/arikawa/v2/state"
	"github.com/diamondburned/arikawa/v2/state/store"
	"github.com/diamondburned/arikawa/v2/voice"
	"github.com/diamondburned/oggreader"
)

// To run, do `APP_ID="APP ID" GUILD_ID="GUILD ID" BOT_TOKEN="TOKEN HERE" go run .`

func main() {
	fmt.Println(os.Getenv("APP_ID"))
	appID := discord.AppID(mustSnowflakeEnv("APP_ID"))
	guildID := discord.GuildID(mustSnowflakeEnv("GUILD_ID"))

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatalln("No $BOT_TOKEN given.")
	}

	s, err := session.New("Bot " + token)
	if err != nil {
		log.Fatalln("Session failed:", err)
		return
	}

	state := state.NewFromSession(s, store.NoopCabinet)
	v, err := voice.NewSession(state)

	s.AddHandler(func(e *gateway.InteractionCreateEvent) {
		data := api.InteractionResponse{
			Type: api.MessageInteractionWithSource,
			Data: &api.InteractionResponseData{
				Content: "Pong!",
			},
		}

		ch, err := s.Channel(812408462382333957)
		v.JoinChannel(ch.GuildID, ch.ID, false, true)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Optimize Opus frame duration. This step is optional, but it is
		udp := v.VoiceUDPConn()
		fmt.Printf("%v\n", udp)
		udp.UseContext(ctx)
		udp.ResetFrequency(60*time.Millisecond, 2880)

		ffmpeg := exec.Command(
			"ffmpeg",
			// Streaming is slow, so a single thread is all we need.
			"-hide_banner", "-threads", "1", "-loglevel", "error",
			// Input file. This should be changed.
			"-i", "266566__gowlermusic__gong-hit.wav",
			// Output format; leave as "libopus".
			"-c:a", "libopus",
			// Bitrate in kilobits. This doesn't matter, but I recommend 96k as the
			// sweet spot.
			"-b:a", "96k",
			// Frame duration should be the same as what's given into
			// udp.ResetFrequency.
			"-frame_duration", "60",
			// Disable variable bitrate to keep packet sizes consistent. This is
			// optional, but it technically reduces stuttering.
			"-vbr", "off",
			// Output format, which is opus, so we need to unwrap the opus file.
			"-f", "opus", "-",
		)

		ffmpeg.Stderr = os.Stderr

		stdout, err := ffmpeg.StdoutPipe()
		if err != nil {
			log.Fatal("failed to get stdout pipe")
		}

		if err := v.Speaking(1); err != nil {
			log.Fatal("failed to send speaking")
		}

		if err := ffmpeg.Start(); err != nil {
			log.Fatal("failed to start ffmpeg")
		}

		if err := oggreader.DecodeBuffered(udp, stdout); err != nil {
			log.Fatal("failed to decode ogg")
		}

		if err := ffmpeg.Wait(); err != nil {
			log.Fatal("failed to finish ffmpeg")
		}

		if err := s.RespondInteraction(e.ID, e.Token, data); err != nil {
			log.Println("failed to send interaction callback:", err)
		}
	})

	s.Gateway.AddIntents(gateway.IntentGuilds)
	s.Gateway.AddIntents(gateway.IntentGuildMessages)

	if err := s.Open(); err != nil {
		log.Fatalln("failed to open:", err)
	}
	defer s.Close()

	log.Println("Gateway connected. Getting all guild commands.")

	commands, err := s.GuildCommands(appID, guildID)
	if err != nil {
		log.Fatalln("failed to get guild commands:", err)
	}

	for _, command := range commands {
		log.Println("Existing command", command.Name, "found.")
	}

	newCommands := []api.CreateCommandData{
		{
			Name:        "start",
			Description: "Start timer",
		},
	}

	for _, command := range newCommands {
		_, err := s.CreateGuildCommand(appID, guildID, command)
		if err != nil {
			log.Fatalln("failed to create guild command:", err)
		}
	}

	// Block forever.
	select {}
}

func mustSnowflakeEnv(env string) discord.Snowflake {
	s, err := discord.ParseSnowflake(os.Getenv(env))
	if err != nil {
		log.Fatalf("Invalid snowflake for $%s: %v", env, err)
	}
	return s
}
