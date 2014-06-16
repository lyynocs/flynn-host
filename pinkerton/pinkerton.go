package pinkerton

import (
	"bytes"
	"errors"
	"net/url"
	"os/exec"
)

func Pull(url string) error {
	return exec.Command("pinkerton", "pull", url).Run()
}

func Checkout(id, image string) (string, error) {
	cmd := exec.Command("pinkerton", "checkout", id, image)
	path, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(path)), nil
}

func Cleanup(id string) error {
	return exec.Command("pinkerton", "cleanup", id).Run()
}

func ImageID(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	q := u.Query()
	id := q.Get("id")
	if id == "" {
		return "", errors.New("pinkerton: missing image id")
	}
	return id, nil
}
