package main

import(
	"github.com/gin-gonic/gin"
	"net/http"
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"time"
	"context"
	amqp "github.com/rabbitmq/amqp091-go"

) 

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

type voteRequest struct {
	NAME string `json:"name"`
}

type VoteDb struct {
	Id int
	Name string
	Creation *time.Time
}

func postVote(c *gin.Context) {
	conn, err := amqp.Dial("amqp://guest:guest@172.17.0.2:5672")
	failOnError(err, "Failed to connect to RabbitMQ")

	ch, err := conn.Channel()
    failOnError(err, "Failed to open a channel")
    defer ch.Close()

	 err = ch.ExchangeDeclare(
                "votes",   // name
                "fanout", // type
                true,     // durable
                false,    // auto-deleted
                false,    // internal
                false,    // no-wait
                nil,      // arguments
    )
    failOnError(err, "Failed to declare an exchange")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var newVote voteRequest

	if err := c.BindJSON(&newVote); err != nil {
		return
	}

	body := newVote.NAME

	err = ch.PublishWithContext(ctx,
			"votes", // exchange
			"",     // routing key
			false,  // mandatory
			false,  // immediate
			amqp.Publishing{
					ContentType: "text/plain",
					Body:        []byte(body),
			})
	failOnError(err, "Failed to publish a message")

	log.Printf(" [x] Sent %s", body)
	
	log.Print("Vote received: " + newVote.NAME)
	c.IndentedJSON(http.StatusCreated, newVote)
}

func getVotes(c *gin.Context) {
	connStr := "postgres://postgres:postgres@localhost/votedb?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	rows, err := db.Query("SELECT id, name, dat_creation FROM votes")

	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	for rows.Next() {
		var vdb VoteDb
		
		if err := rows.Scan(&vdb.Id, &vdb.Name, &vdb.Creation); err != nil {
			log.Fatalf("error %v: %q", &vdb.Name, err)
		}

		log.Print("vote detail:\n Id: ", vdb.Id, "Name: ", vdb.Name, "Creation: ", vdb.Creation.String())
	}

	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	router := gin.Default()
	router.POST("/api/votes", postVote)
	router.GET("/api/votes", getVotes)
	router.Run()
}
