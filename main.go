package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"mvdan.cc/xurls/v2"
)

var scheduledEvents map[string]bool
var l sync.Mutex

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func slackNotify(source string, organizer string, email string, doc string) error {
	text := "Hey there! You have a scheduled one-on-one meeting with %s in 15 minutes. The agenda for the meeting is:\n• check-in\n• your past five days of work (successes, blockers, ...)\n• teamwork\n• company\n• anything else on your mind?\n\nPlease check out your one-on-one document at %s before attending the meeting.\n\nMeeting link: %s\n\nSee you soon :)"

	api := slack.New(os.Getenv("SLACK_TOKEN"))
	users, err := api.GetUsers()
	if err != nil {
		return err
	}
	idx := strings.Index(email, "@")
	email = email[:idx]
	for _, user := range users {
		if strings.Contains(user.Name, email) || strings.Contains(user.Profile.Email, email) {
			fmt.Printf("notifying %q ...\n", user.Name)
			_, _, err := api.PostMessage(user.ID, slack.MsgOptionText(fmt.Sprintf(text, organizer, doc, source), true))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func handleOneOnOne(organizer string, item *calendar.Event) {
	date := item.Start.DateTime
	if date == "" {
		date = item.Start.Date
	}
	idate, err := time.Parse(time.RFC3339, date)
	if err != nil {
		fmt.Printf("Error handling time: %v\n", err)
		return
	}
	l.Lock()
	scheduled := scheduledEvents[item.Id]
	if scheduled {
		l.Unlock()
		return
	}
	scheduledEvents[item.Id] = true
	l.Unlock()

	docUrl := xurls.Strict().FindAllString(item.Description, 1)
	if len(docUrl) == 0 {
		docUrl = append(docUrl, "(no document available)")
	}
	source := item.HtmlLink
	for _, attendee := range item.Attendees {
		if attendee.Email == organizer {
			if attendee.ResponseStatus == "declined" {
				return
			}
			continue
		}
		if attendee.ResponseStatus == "declined" {
			continue
		}
		if strings.Contains(attendee.Email, "calendar.google.com") {
			continue
		}

		go func(attendee *calendar.EventAttendee) {
			defer func() {
				l.Lock()
				delete(scheduledEvents, item.Id)
				l.Unlock()
			}()
			d := idate.Sub(time.Now())
			if d < 0 {
				return
			}
			fmt.Printf("(will notify user %v in %v)\n", attendee.Email, d)
			if d > 15*time.Minute {
				d -= 15 * time.Minute
				time.Sleep(d)
			}
			if err := slackNotify(source, organizer, attendee.Email, docUrl[0]); err != nil {
				fmt.Printf("Error notifying user %s: %v\n", attendee.Email, err)
			}
		}(attendee)
	}
}

func main() {
	flagOrganizer := flag.String("o", "john@example.com", "organizer")
	flag.Parse()
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	scheduledEvents = make(map[string]bool)

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	for {
		t := time.Now().Format(time.RFC3339)
		events, err := srv.Events.List("primary").ShowDeleted(false).
			SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
		if err != nil {
			log.Fatalf("Unable to retrieve next ten of the user's events: %v", err)
		}
		fmt.Println("Upcoming events:")
		if len(events.Items) == 0 {
			fmt.Println("No upcoming events found.")
		} else {
			for _, item := range events.Items {
				if strings.Contains(item.Summary, "1:1") && item.Organizer.Email == *flagOrganizer {
					fmt.Printf("Got scheduled one on ones ...\n")
					handleOneOnOne(*flagOrganizer, item)
				}
			}
		}
		time.Sleep(time.Hour)
	}
}
