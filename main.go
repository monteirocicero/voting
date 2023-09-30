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
	Name string
	QtdVotes int
}

type VoteResponse struct {
	Name string `json:"name"`
	Votes float64 `json:"votes"`
}

func calculate(votedb []VoteDb) ([]VoteResponse) {

	var total = 0
	var result []VoteResponse
	

	for _, vote := range votedb {
		total += vote.QtdVotes
	}

	for _, vote := range votedb {
		println(vote.Name)
		println(vote.QtdVotes)
		v := VoteResponse{Name: vote.Name, Votes: (float64(vote.QtdVotes)/float64(total))*100 }

		result = append(result, v)
	}

	return result

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
				    DeliveryMode: amqp.Persistent,
					ContentType: "text/plain",
					Body:        []byte(body),
			})
	failOnError(err, "Failed to publish a message")

	log.Printf(" [x] Sent %s", body)
	
	log.Print("Vote received: " + newVote.NAME)
	c.IndentedJSON(http.StatusCreated, newVote)
}

func getVotes(c *gin.Context) {
	connStr := "postgres://postgres:postgres@172.17.0.3/votedb?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	rows, err := db.Query("SELECT name, count(*) as qtdVotes FROM public.votes GROUP by name")

	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	var votes []VoteDb

	for rows.Next() {
		var vdb VoteDb
		
		if err := rows.Scan(&vdb.Name, &vdb.QtdVotes); err != nil {
			log.Fatalf("error %v: %q", &vdb.Name, err)
		}

		votes = append(votes, vdb)
		
	}

	calculate(votes)

	
	c.IndentedJSON(http.StatusCreated, calculate(votes))
}

func main() {
	router := gin.Default()
	router.POST("/api/votes", postVote)
	router.GET("/api/votes", getVotes)
	router.Run()
}
