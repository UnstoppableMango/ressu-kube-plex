package kubernetes

import (
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client"

	log "github.com/Sirupsen/logrus"

	"github.com/munnerz/plex-elastic-transcoder/executors"
	"github.com/munnerz/plex-elastic-transcoder/common"
)

const podBasename = "plex-transcoder"
const kubernetesHost = "10.20.40.254:8080"
const kubernetesNamespace = "plex"
const dockerImage = "registry.marley.xyz/e720/plex-new-transcoder"

type KubernetesExecutor struct {
	executors.AbstractExecutor

	Host       string
	Namespace  string
	Image      string

	pod        *api.Pod
	client     *client.Client
}

func (e *KubernetesExecutor) createPod() *api.Pod {
	return &api.Pod{
				TypeMeta: api.TypeMeta{
					Kind: "Pod",
				},
				ObjectMeta: api.ObjectMeta{
					GenerateName: podBasename,
					Namespace: e.Namespace,
				},
				Spec: api.PodSpec{
					RestartPolicy: api.RestartPolicyNever,
					Containers: []api.Container{
						api.Container{
							Name: podBasename,
							Image: e.Image,
							Command: e.Job.Command,
							Args: e.Job.Args,
						},
					},
				},
			}
}

func (e *KubernetesExecutor) Start() error {
	log.Print("Executing job: ", e)

	e.pod = e.createPod()

	client, err := client.New(
		&client.Config{
			Host: e.Host,
		})

	if err != nil {
		return err
	}

	e.client = client

	log.Print("Creating pod...")
	pod, err := e.client.Pods(e.pod.ObjectMeta.Namespace).Create(e.pod)

	if err != nil {
		return err
	}

	e.pod = pod

	log.Print("Successfully scheduled job on cluster with name: ", e.pod.ObjectMeta.Name)

	return nil
}

func (e *KubernetesExecutor) Stop() error {
	return e.client.Pods(e.pod.ObjectMeta.Namespace).Delete(e.pod.ObjectMeta.Name, nil)
}

func podPhaseToExecutorPhase(in api.PodPhase) executors.ExecutorPhase {
	switch in {
	case api.PodRunning:
		return executors.ExecutorRunning
	case api.PodPending:
		return executors.ExecutorPreparing
	case api.PodSucceeded:
		return executors.ExecutorSucceeded
	case api.PodFailed:
		return executors.ExecutorFailed
	default:
		return executors.ExecutorUnknown
	}
}

func (e *KubernetesExecutor) WaitForState(targetState executors.ExecutorPhase) error {
	Loop:
	for {
		pod, err := e.client.Pods(e.pod.ObjectMeta.Namespace).Get(e.pod.ObjectMeta.Name)
		if err != nil {
			return err
		}

		Switch:
		switch podPhaseToExecutorPhase(pod.Status.Phase) {
		case targetState:
			break Loop
		default:
			break Switch
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func init() {
	common.RegisterExecutor("kubernetes", common.ExecutorFactory{
		Create: func(j executors.Job) executors.Executor {
			return &KubernetesExecutor{
				Host: kubernetesHost,
				Namespace: kubernetesNamespace,
				Image: dockerImage,
			}
		},
	})
}