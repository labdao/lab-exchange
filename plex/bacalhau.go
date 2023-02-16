package main

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/bacalhau/pkg/downloader"
	"github.com/filecoin-project/bacalhau/pkg/downloader/util"
	"github.com/filecoin-project/bacalhau/pkg/model"
	"github.com/filecoin-project/bacalhau/pkg/requester/publicapi"
	"github.com/filecoin-project/bacalhau/pkg/system"
)

func createBacalhauJob(cid, container, cmd string) (job *model.Job, err error) {
	job, err = model.NewJobWithSaneProductionDefaults()
	if err != nil {
		return nil, err
	}
	job.Spec.Engine = model.EngineDocker
	job.Spec.Publisher = model.PublisherIpfs
	job.Spec.Docker.Image = container
	job.Spec.Docker.Entrypoint = []string{"/bin/bash", "-c", cmd}
	job.Spec.Network = model.NetworkConfig{Type: model.NetworkFull}
	job.Spec.Resources.Memory = "12gb"
	job.Spec.Resources.GPU = "1"
	job.Spec.Inputs = []model.StorageSpec{{StorageSource: model.StorageSourceIPFS, CID: cid, Path: "/inputs"}}
	job.Spec.Outputs = []model.StorageSpec{{Name: "outputs", StorageSource: model.StorageSourceIPFS, Path: "/outputs"}}
	return job, err
}

func submitBacalhauJob(job *model.Job) (submittedJob *model.Job, err error) {
	system.InitConfig()
	apiPort := 1234
	apiHost := "35.245.115.191"
	client := publicapi.NewRequesterAPIClient(fmt.Sprintf("http://%s:%d", apiHost, apiPort))
	submittedJob, err = client.Submit(context.Background(), job)
	return submittedJob, err
}

func getBacalhauJobResults(submittedJob *model.Job) (results []model.PublishedResult, err error) {
	system.InitConfig()
	apiPort := 1234
	apiHost := "35.245.115.191"
	client := publicapi.NewRequesterAPIClient(fmt.Sprintf("http://%s:%d", apiHost, apiPort))
	maxTrys := 360 // 30 minutes divided by 5 seconds is 360 iterations
	for i := 0; i < maxTrys; i++ {
		jobState, err := client.GetJobState(context.Background(), submittedJob.Metadata.ID)
		if err != nil {
			return results, err
		}

		// check to see if any node shards have finished or errored while running job
		// this assumes the job spec will only have one shard attempt to run the job
		completedShardRuns := []model.JobShardState{}
		erroredShardRuns := []model.JobShardState{}
		for _, jobNodeState := range jobState.Nodes {
			for _, jobShardState := range jobNodeState.Shards {
				if jobShardState.State == model.JobStateCompleted {
					completedShardRuns = append(completedShardRuns, jobShardState)
				} else if jobShardState.State == model.JobStateError {
					erroredShardRuns = append(erroredShardRuns, jobShardState)
				}
			}
		}
		if len(completedShardRuns) > 0 || len(erroredShardRuns) > 0 {
			fmt.Println("Job run complete")
			results, err = client.GetResults(context.Background(), submittedJob.Metadata.ID)
			return results, err
		}
		fmt.Println("Job still running...")
		time.Sleep(5 * time.Second)
	}
	return results, err
}

func DownloadBacalhauResults(dir string, submittedJob *model.Job, results []model.PublishedResult) error {
	downloadSettings := util.NewDownloadSettings()
	downloadSettings.OutputDir = dir
	cm := system.NewCleanupManager()
	downloaderProvider := util.NewStandardDownloaders(cm, downloadSettings)
	err := downloader.DownloadJob(context.Background(), submittedJob.Spec.Outputs, results, downloaderProvider, downloadSettings)
	return err
}
