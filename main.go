package main

import (
	"context"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/bwmarrin/discordgo"

	"github.com/dghubble/go-twitter/twitter"
)

var reTweets = regexp.MustCompile(`https?://(?:mobile\.|www\.)?twitter.com/\w+/status/(\d+)`)

func mustGetenv(name string) string {
	val := os.Getenv(name)
	if len(val) == 0 {
		log.Fatalf("expected %v to be set", name)
	}
	return val
}

func getTweetImages(tw *twitter.Client, id int64) ([]string, error) {
	tweet, _, err := tw.Statuses.Show(id, &twitter.StatusShowParams{})
	if err != nil {
		return nil, err
	}
	if tweet.ExtendedEntities == nil {
		log.Println("tweet.ExtendedEntities == nil")
		return nil, nil
	}
	urls := []string{}
	for _, media := range tweet.ExtendedEntities.Media {
		urls = append(urls, media.MediaURLHttps)
	}
	return urls, err
}

func main() {
	ctx := context.Background()

	twitterKey := mustGetenv("TWITTER_CONSUMER_KEY")
	twitterSecret := mustGetenv("TWITTER_CONSUMER_SECRET")
	config := &clientcredentials.Config{
		ClientID:     twitterKey,
		ClientSecret: twitterSecret,
		TokenURL:     "https://api.twitter.com/oauth2/token",
	}
	httpClient := config.Client(oauth2.NoContext)
	tw := twitter.NewClient(httpClient)

	discordToken := mustGetenv("DISCORD_TOKEN")

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalf("couldn't create discord client: %v\n", err)
	}

	dg.AddHandler(func(s *discordgo.Session, event *discordgo.Ready) {
	})

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		channel, err := s.State.Channel(m.ChannelID)
		if err != nil {
			log.Printf("couldn't get channel %v: %v\n", m.ChannelID, err)
		}
		if channel.Name != "bots" {
			log.Printf("channel %v isn't bots, ignoring\n", channel.Name)
		}
		tweets := reTweets.FindAllStringSubmatch(m.Content, -1)
		tweetIDs := []int64{}
		tweetIDsMap := map[int64]struct{}{}
		for _, tweet := range tweets {
			if len(tweet) < 2 {
				log.Printf("len(tweet) < 2: %v, msg = %#v\n", tweet, m)
				continue
			}
			tweetID, err := strconv.ParseInt(tweet[1], 10, 64)
			if err != nil {
				log.Printf("couldn't parse tweet id: %v, msg = %#v\n", tweet, m)
				continue
			}
			if _, ok := tweetIDsMap[tweetID]; ok {
				continue
			}
			tweetIDs = append(tweetIDs, tweetID)
			tweetIDsMap[tweetID] = struct{}{}
		}

		for _, tweetID := range tweetIDs {
			urls, err := getTweetImages(tw, tweetID)
			if err != nil {
				log.Printf("couldn't get tweet images: %v\n", err)
				continue
			}
			if len(urls) < 2 {
				continue
			}
			if len(urls[0]) < len("https://") {
				log.Println("first url seems too short: %v\n", urls[0])
				continue
			}
			urls[0] = urls[0][len("https://"):]
			urlsMsg := strings.Join(urls, " ")
			_, err = s.ChannelMessageSend(channel.ID, urlsMsg)
			if err != nil {
				log.Printf("error sending \"%v\": %v", urlsMsg, err)
			}
		}
	})

	if err := dg.Open(); err != nil {
		log.Fatalf("couldn't connect to discord: %v\n", err)
	}

	<-ctx.Done()
}
