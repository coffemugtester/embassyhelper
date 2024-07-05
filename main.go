package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/gocolly/colly"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"strings"
)

type Embassy struct {
	HomeCountry string `bson:"home_country"`
	HostCountry string `bson:"host_country"`
	Name        string `bson:"name"`
	MapLink     string `bson:"map_link"`
	City        string `bson:"city"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// var apiKey = os.Getenv("API_KEY")

	//TODO: create mongo client with methods implemented to it
	collection := getMongoCollection()

	homeCountry, hostCountry := parseFlags()
	fmt.Printf("Countries\nHome: %s\nHost: %s\n", *hostCountry, *homeCountry)

	embassies := scrapeEmbassyList(*homeCountry, *hostCountry)

	for _, embassy := range embassies {
		//TODO: create google client to include google id in the embassy struct
		//TODO: check if embassy already exists
		_, err := collection.InsertOne(context.TODO(), embassy)
		if err != nil {
			fmt.Printf("Error inserting embassy: %v", err)
			return
		}
		fmt.Printf("Inserted embassy: %v\n", embassy)
	}

}

func parseFlags() (homeCountry, hostCountry *string) {
	homeCountry = flag.String("home", "", "The country where the embassy is located")
	hostCountry = flag.String("host", "", "The country represented by the embassy")

	flag.Parse()

	if *homeCountry == "" || *hostCountry == "" {
		fmt.Println("Both home and host country are required.")
		flag.Usage()
		os.Exit(1)
	}

	if *homeCountry == *hostCountry {
		//TODO: if there's an array of countries - skip if home and host country are the same
		fmt.Println("Home and host country cannot be the same.")
		flag.Usage()
		os.Exit(1)
	}

	return homeCountry, hostCountry
}

func scrapeEmbassyList(homeCountry, hostCountry string) []Embassy {
	//TODO: take in an array of countries

	var names, mapLinks []string
	var embassiesAmount int

	c := colly.NewCollector()

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		embassiesAmount = len(e.DOM.Find("h2").Nodes)

		e.ForEach("h2", func(_ int, el *colly.HTMLElement) {
			name := el.Text
			name = strings.Replace(name, "\n ", "", -1)
			names = append(names, name)
		})

		e.ForEach("div.embassy__map", func(_ int, el *colly.HTMLElement) {
			// Find the map link
			mapLink := el.ChildAttr("a.embassy__map-link", "href")
			mapLink = strings.Replace(mapLink, "\n", "", -1)
			mapLinks = append(mapLinks, mapLink)
		})
	})

	// Start scraping on the given URL
	err := c.Visit(fmt.Sprintf("https://www.visahq.de/en/%s/embassy/%s/", homeCountry, hostCountry))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	embassies := make([]Embassy, embassiesAmount)
	for i := range names {
		if i < len(mapLinks) {
			cityName, err := getCityName(names[i])
			if err != nil {
				fmt.Printf("Error getting city name: %v", err)
			}

			embassies[i] = Embassy{
				HomeCountry: homeCountry,
				HostCountry: hostCountry,
				Name:        names[i],
				MapLink:     mapLinks[i],
				City:        cityName,
			}
		}
	}
	return embassies
}

// getCityName extracts the city name from a string in the format "New house in CityName"
func getCityName(input string) (string, error) {
	parts := strings.Split(input, " in ")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid input string format")
	}
	cityName := strings.TrimSpace(parts[1])
	return cityName, nil
}

func getMongoCollection() *mongo.Collection {

	var mongoURI = os.Getenv("MONGO_URI")
	var mongoDB = os.Getenv("MONGO_DB")
	var mongoCollection = os.Getenv("MONGO_COLLECTION")

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoURI).SetServerAPIOptions(serverAPI))
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		fmt.Printf("Error connecting to MongoDB: %v", err)
		return nil
	}

	collection := client.Database(mongoDB).Collection(mongoCollection)
	return collection
}
