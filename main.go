package main

import (
	"context"
	"log"
	"time"
)

type callback struct {
}

func (cb *callback) OnStartLead(context.Context) {
	log.Println("start leading")
}

func (cb *callback) OnStopLead() {
	log.Println("stop leading")
}
func (cb *callback) OnNewLead(leader string) {
	log.Printf("now leader is %s", leader)
}

func main() {
	elect, err := NewK8sElect(context.Background(), K8sElectConfig{Callbacks: &callback{}, LockName: "test-lock"})
	if err != nil {
		panic(err)
	}
	for {
		if elect.IsLeader() {
			time.Sleep(15 * time.Second)
			elect.LeaderResign()
		} else {
			time.Sleep(time.Second)
		}
	}
}
