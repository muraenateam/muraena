package db

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"log"
)

type WhoisLookup struct {
	Fqdn        string `redis:"-" json:"fqdn"`
	Nameservers string `redis:"ns" json:"ns"`
	CreatedAt   string `redis:"createdAt" json:"createdAt"`
	Emails      string `redis:"emails" json:"emails"`

	SQNameservers string `redis:"sqns" json:"sqns"`
	SQCreatedAt   string `redis:"sqcreatedAt" json:"sqcreatedAt"`
	SQEmails      string `redis:"sqemails" json:"sqemails"`
}

func GetWhoisLookup(domain string) (*WhoisLookup, error) {

	rc := RedisPool.Get()
	defer rc.Close()

	var who WhoisLookup
	key := fmt.Sprintf("whois:lookup:%s", domain)

	value, err := redis.Values(rc.Do("HGETALL", key))
	if err != nil {
		log.Printf("error getting whois lookup from redis: %s", err)
		return nil, err
	}

	err = redis.ScanStruct(value, &who)
	if err != nil {
		log.Printf("error scanning whois struct lookup from redis: %s", err)
		return nil, err
	}

	return &who, nil
}

func AddWhoisLookup(lookup *WhoisLookup) error {
	rc := RedisPool.Get()
	defer rc.Close()

	// TODO don't add if it exists! DO A HGET first

	key := fmt.Sprintf("whois:lookup:%s", lookup.Fqdn)
	_, err := rc.Do("HMSET", key,
		"ns", lookup.Nameservers,
		"createdAt", lookup.CreatedAt,
		"emails", lookup.Emails,
		// sq means secondaryQuery (might be empty)
		"sqns", lookup.SQNameservers,
		"sqcreatedAt", lookup.SQCreatedAt,
		"sqemails", lookup.Emails,
	)
	if err != nil {
		return err
	}

	return nil
}
