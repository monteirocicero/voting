package main

import (
	"log"
	"database/sql"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func save(name string) {
	 connStr := "postgres://postgres:postgres@172.17.0.3/votedb?sslmode=disable"
	 db, err := sql.Open("postgres", connStr)

	 if err != nil {
	 	log.Fatal(err)
	}

	result, err := db.Query("INSERT INTO public.votes(name, dat_creation) VALUES($1, CURRENT_TIMESTAMP)", name)

	if err != nil {
		log.Fatal(err)
	}
	
	println(result)
}

func main() {

	conn, err := amqp.Dial("amqp://guest:guest@172.17.0.2:5672")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

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

	q, err := ch.QueueDeclare(
		"votes_queue",    // name
		true, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.QueueBind(
		q.Name, // queue name
		"",     // routing key
		"votes", // exchange
		false,
		nil,
	)
	failOnError(err, "Failed to bind a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf(" [x] %s", d.Body)
			save(string(d.Body))
			d.Ack(false)
		}
	}()

	log.Printf(" [*] Waiting for votes.")
	<-forever
}
