package db

import (
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/oschwald/maxminddb-golang"
	"log"
	"time"
)

var (
	host     = "127.0.0.1"
	port     = 6379
	password = ""

	RedisPool *redis.Pool

	ipQueueKey = "ip-queue"

	maxMindDB *maxminddb.Reader

	QueuePollDelay = 100 * time.Millisecond
)

func newRedisPool(host string, port int, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}

			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func Init() error {
	// init a new Redis db Pool
	RedisPool = newRedisPool(host, port, password)
	if RedisPool == nil {
		return errors.New("error connecting to redis")
	}

	// TODO check if maxmind is enabled in config first
	db, err := maxminddb.Open("config/GeoLite2-City.mmdb")
	if err != nil {
		log.Printf("error loading MaxMind DB (worker/GeoLite2-City.mmdb): %v", err)
		return err
	}
	maxMindDB = db

	return nil
}
