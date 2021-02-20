package main

import (
	"fmt"
	"log"
	"os"

	"github.com/diamondburned/arikawa/v2/api"
	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/diamondburned/arikawa/v2/gateway"
	"github.com/diamondburned/arikawa/v2/session"
	"github.com/diamondburned/arikawa/v2/state"
	"github.com/diamondburned/arikawa/v2/state/store"
	"github.com/diamondburned/arikawa/v2/voice"
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

		v.JoinChannel(812408462382333953, 812408462382333957, false, true)

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
