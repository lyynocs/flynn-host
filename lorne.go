package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"io"
	"log"
	"os"

	"github.com/flynn/rpcplus"
	"github.com/flynn/sampi/types"
	"github.com/titanous/go-dockerclient"
)

var state = NewState()

func main() {
	scheduler, err := rpcplus.DialHTTP("tcp", "localhost:1112")
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Connected to scheduler")

	client, err := docker.NewClient("http://localhost:4243")
	if err != nil {
		log.Fatal(err)
	}

	go streamEvents(client)
	go syncScheduler(scheduler)

	host := sampi.Host{
		ID:        randomID(),
		Resources: map[string]sampi.ResourceValue{"memory": sampi.ResourceValue{Value: 1024}},
	}

	jobs := make(chan *sampi.Job)
	scheduler.StreamGo("Scheduler.RegisterHost", host, jobs)
	log.Print("Host registered")
	for job := range jobs {
		log.Printf("%#v", job.Config)
		state.AddJob(job)
		container, err := client.CreateContainer(job.Config)
		if err == docker.ErrNoSuchImage {
			err = client.PullImage(docker.PullImageOptions{Repository: job.Config.Image}, os.Stdout)
			if err != nil {
				log.Fatal(err)
			}
			container, err = client.CreateContainer(job.Config)
		}
		if err != nil {
			log.Fatal(err)
		}
		state.SetContainerID(job.ID, container.ID)
		if err := client.StartContainer(container.ID); err != nil {
			log.Fatal(err)
		}
		state.SetStatusRunning(job.ID)
	}
}

func syncScheduler(client *rpcplus.Client) {
	events := make(chan Event)
	state.AddListener("all", events)
	for event := range events {
		if event.Event != "stop" {
			continue
		}
		log.Println("remove job", event.JobID)
		if err := client.Call("Scheduler.RemoveJobs", []string{event.JobID}, &struct{}{}); err != nil {
			log.Println("remove job", event.JobID, "error:", err)
			// TODO: try to reconnect?
		}
	}
}

func streamEvents(client *docker.Client) {
	stream, err := client.Events()
	if err != nil {
		log.Fatal(err)
	}
	for event := range stream.Events {
		log.Printf("%#v", event)
		if event.Status != "die" {
			continue
		}
		container, err := client.InspectContainer(event.ID)
		if err != nil {
			log.Println("inspect container", event.ID, "error:", err)
			// TODO: set job status anyway?
			continue
		}
		state.SetStatusDone(event.ID, container.State.ExitCode)
	}
	log.Println("events done", stream.Error)
}

func randomID() string {
	b := make([]byte, 16)
	enc := make([]byte, 24)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		panic(err) // This shouldn't ever happen, right?
	}
	base64.URLEncoding.Encode(enc, b)
	return string(bytes.TrimRight(enc, "="))
}