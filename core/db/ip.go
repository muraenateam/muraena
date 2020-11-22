package db

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"log"
	"net"
	"time"
)

type MaxMindLookup struct {
	Ip          string `redis:"ip" json:"ip"`
	Country     string `redis:"country" json:"country"`
	City        string `redis:"city" json:"city"`
	Timezone    string `redis:"tz" json:"tz"`
	Geolocation string `redis:"geo" json:"geo"`
	Time        string `redis:"time" json:"time"`
}

type MaxMindEntry struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names struct {
			En string `maxminddb:"en"`
		} `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		Timezone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
}

func QueryMaxMind(queryIp string) *MaxMindLookup {
	var maxmindEntry MaxMindEntry

	ip := net.ParseIP(queryIp)

	//log.Printf("querying MaxMind for IP: %s", ip)

	err := maxMindDB.Lookup(ip, &maxmindEntry)
	if err != nil {
		log.Printf(fmt.Sprintf("error doing maxmind lookup for IP %s: %s", ip.String(), err))
	}

	//// debugging code ...
	//var genericRecord interface{}
	//_ = maxMindDB.Lookup(ip, &genericRecord)
	//log.Printf(fmt.Sprintf("LOGGING FULL MAXMIND DB CONTENT FOR IP %s:\n%+v", ip.String(),genericRecord))

	if len(maxmindEntry.City.Names.En) < 2 {
		maxmindEntry.City.Names.En = "-"
	}

	result := MaxMindLookup{
		Ip:       ip.String(),
		Country:  maxmindEntry.Country.ISOCode,
		City:     maxmindEntry.City.Names.En,
		Timezone: maxmindEntry.Location.Timezone,
		Geolocation: fmt.Sprintf("https://www.google.com/maps/place/%f,%f",
			maxmindEntry.Location.Latitude, maxmindEntry.Location.Longitude),
	}

	return &result
}

func AddMaxMindLookup(lookup *MaxMindLookup) error {
	rc := RedisPool.Get()
	defer rc.Close()

	// TODO don't add if it exists! DO A HGET first

	key := fmt.Sprintf("maxmind:lookup:%s", lookup.Ip)
	_, err := rc.Do("HMSET", key,
		"country", lookup.Country,
		"city", lookup.City,
		"tz", lookup.Timezone,
		"geo", lookup.Geolocation,
		"time", time.Now().String(),
	)
	if err != nil {
		return err
	}

	return nil
}

func GetMaxmindLookup(ip string) (*MaxMindLookup, error) {
	rc := RedisPool.Get()
	defer rc.Close()

	var lookup MaxMindLookup
	key := fmt.Sprintf("maxmind:lookup:%s", ip)

	value, err := redis.Values(rc.Do("HGETALL", key))
	if err != nil {
		return nil, err
	}

	err = redis.ScanStruct(value, &lookup)
	if err != nil {
		return nil, err
	}

	return &lookup, nil
}

func PopIpQueue() (string, error) {
	rc := RedisPool.Get()
	defer rc.Close()
	timeout := 0

	pop, err := redis.Strings(rc.Do("BRPOP", ipQueueKey, timeout))
	if err != nil {
		log.Printf("error popping from IP queue: %s", err)
		return "", err
	}

	return pop[1], nil
}

func PushIpQueue(ip string) {
	rc := RedisPool.Get()
	defer rc.Close()

	if _, err := rc.Do("LPUSH", ipQueueKey, ip); err != nil {
		log.Printf("error pushing ip(%s) to IP queue: %s", ip, err)
	}
}
