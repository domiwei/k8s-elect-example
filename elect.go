package main

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

const (
	envPodName   = "POD_NAME"
	envNamespace = "POD_NAMESPACE"
)

type Elect interface {
	GetLeader() string
	IsLeader() bool
	LeaderResign() error
	Terminate()
}

type CallBacks interface {
	OnStartLead(context.Context)
	OnStopLead()
	OnNewLead(string)
}

type emptyCallBack struct{}

func (e emptyCallBack) OnStartLead(ctx context.Context) {}
func (e emptyCallBack) OnStopLead()                     {}
func (e emptyCallBack) OnNewLead(id string)             {}

type k8sImpl struct {
	leConfig leaderelection.LeaderElectionConfig
	le       *leaderelection.LeaderElector
	resign   atomic.Value
	stop     func()
}

type K8sElectConfig struct {
	Callbacks CallBacks
	LockName  string
}

func NewK8sElect(ctx context.Context, inputConfig K8sElectConfig) (Elect, error) {
	config, err := rest.InClusterConfig()
	client := clientset.NewForConfigOrDie(config)
	if err != nil {
		return nil, err
	}
	leaseLockName := inputConfig.LockName
	leaseLockNamespace := os.Getenv(envNamespace)
	podname := os.Getenv(envPodName)

	log.Printf("namespace: %s, podname: %s", leaseLockNamespace, podname)

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: leaseLockNamespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podname,
		},
	}

	cb := inputConfig.Callbacks
	if cb == nil {
		cb = emptyCallBack{}
	}

	leConfig := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: cb.OnStartLead,
			OnStoppedLeading: cb.OnStopLead,
			OnNewLeader:      cb.OnNewLead,
		},
		WatchDog:        &leaderelection.HealthzAdaptor{},
		ReleaseOnCancel: true,
		Name:            "",
	}

	le, err := leaderelection.NewLeaderElector(leConfig)
	if err != nil {
		return nil, err
	}

	cctx, cancel := context.WithCancel(ctx)
	im := &k8sImpl{
		leConfig: leConfig,
		le:       le,
		stop:     func() { cancel() },
	}
	go im.run(cctx)
	return im, nil
}

func (im *k8sImpl) run(ctx context.Context) {
	for {
		leCtx, cancel := context.WithCancel(ctx)
		im.resign.Store(func() { cancel() })
		im.le.Run(leCtx)
		select {
		case <-time.NewTicker(3 * time.Second).C:
			log.Println("wake up")
			// Sleep a while before get involved into next leader election
		case <-ctx.Done():
			// the end of this elect object
			return
		}
	}
}

func (im *k8sImpl) GetLeader() string {
	return im.le.GetLeader()
}
func (im *k8sImpl) IsLeader() bool {
	return im.le.IsLeader()
}

func (im *k8sImpl) LeaderResign() error {
	if im.IsLeader() {
		// Take no effect if it's not a leader
		if f := im.resign.Load(); f != nil {
			f.(func())()
		}
	}
	return nil
}

func (im *k8sImpl) Terminate() {
	im.stop()
}
