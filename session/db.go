package session

import (
	"errors"
	"fmt"
	"time"

	"github.com/evilsocket/islazy/tui"
	"github.com/gomodule/redigo/redis"
)

var (
	host     = "127.0.0.1"
	port     = 6379
	password = ""

	RedisPool *redis.Pool
)

// InitRedis initialize the connection to a Redis database
func (s *Session) InitRedis() error {

	var config = s.Config.Redis

	if config.Host != "" {
		host = config.Host
	}

	if config.Password != "" {
		password = config.Password
	}

	if config.Port != 0 {
		port = config.Port
	}

	// init a new Redis db Pool
	RedisPool = newRedisPool()
	if _, err := RedisPool.Dial(); err != nil {
		return errors.New(fmt.Sprintf("%s %s", tui.Wrap(tui.BACKLIGHTBLUE, tui.Wrap(tui.FOREBLACK, "redis")), err.Error()))
	}

	return nil
}

func newRedisPool() *redis.Pool {

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
