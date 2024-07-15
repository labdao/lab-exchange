package ray

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/labdao/plex/gateway/models"
	"github.com/labdao/plex/internal/ipwl"
	"gorm.io/gorm"
)

var rayClient *http.Client
var once sync.Once

func GetRayApiHost() string {
	// For colabfold local testing set this env var to http://colabfold-service:<PORT>
	rayApiHost, exists := os.LookupEnv("RAY_API_HOST")
	if exists {
		return rayApiHost
	} else {
		return "localhost:8000" // Default Ray API host
	}
}

func GetRayJobApiHost() string {
	// For colabfold local testing set this env var to http://colabfold-service:<PORT>
	rayApiHost, exists := os.LookupEnv("RAY_JOB_API_HOST")
	if exists {
		return rayApiHost
	} else {
		return "localhost:8265" // Default Ray API host
	}
}

// Prevents race conditions with Ray Client
func GetRayClient() *http.Client {
	once.Do(func() {
		rayClient = &http.Client{}
	})
	return rayClient
}

func handleSingleElementInput(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case float64, int, int64:
		// Convert numeric values to string
		return fmt.Sprintf("%v", v), nil
	default:
		return "", fmt.Errorf("unsupported type: %T", v)
	}
}

func CreateRayJob(job *models.Job, modelPath string, rayJobID string, inputs map[string]interface{}, db *gorm.DB) (*http.Response, error) {
	model, _, err := ipwl.ReadModelConfig(modelPath, db)
	if err != nil {
		return nil, err
	}
	var jsonBytes []byte
	var rayServiceURL string
	if job.JobType == models.JobTypeService {
		// Validate input keys
		err = validateInputKeys(inputs, model.Inputs)
		if err != nil {
			return nil, err
		}

		adjustedInputs := make(map[string]string)
		for key, value := range inputs {
			switch v := value.(type) {
			case []interface{}:
				if len(v) == 1 {
					adjustedInputs[key], err = handleSingleElementInput(v[0])
					if err != nil {
						return nil, fmt.Errorf("invalid input for key %s: %v", key, err)
					}
				} else {
					return nil, fmt.Errorf("expected a single-element slice for key %s, got: %v", key, v)
				}
			case string, float64, int:
				adjustedInputs[key], err = handleSingleElementInput(value)
				if err != nil {
					return nil, fmt.Errorf("invalid input for key %s: %v", key, err)
				}
			default:
				return nil, fmt.Errorf("unsupported type for key %s: %T", key, value)
			}
		}

		//add rayJobID to inputs
		fmt.Printf("adding rayJobID to the adjustedInputs: %s\n", rayJobID)
		adjustedInputs["uuid"] = rayJobID

		// Marshal the inputs to JSON
		jsonBytes, err := json.Marshal(adjustedInputs)
		if err != nil {
			return nil, err
		}

		log.Printf("Submitting Ray job with payload: %s\n", string(jsonBytes))

		// construct from env var BUCKET ENDPOINT + model.RayEndpoint
		rayServiceURL = GetRayApiHost() + model.RayEndpoint
		// Create the HTTP request

	} else if job.JobType == models.JobTypeJob {

		rayServiceURL = GetRayJobApiHost() + model.RayEndpoint

		runtimeEnv := map[string]interface{}{
			"env_vars": map[string]string{
				"REQUEST_UUID": rayJobID,
			},
		}
		// create json request body
		reqBody := map[string]interface{}{
			"entrypoint":    model.RayJobEntrypoint,
			"job_id":        rayJobID,
			"submission_id": rayJobID,
			"runtime_env":   runtimeEnv,
		}
		jsonBytes, err = json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}

	}
	req, err := http.NewRequest("POST", rayServiceURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request to the Ray service
	client := GetRayClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func GetRayJobStatus(rayJobID string) (string, error) {
	rayServiceURL := GetRayJobApiHost() + "/api/jobs/" + rayJobID
	req, err := http.NewRequest("GET", rayServiceURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request to the Ray service
	client := GetRayClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	log.Printf("Ray job status response: %s\n", string(body))
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Fatalf("Error parsing JSON: %s", err)
	}

	status, ok := data["status"]
	if !ok {
		log.Fatal("Status field not found")
	}
	return status.(string), nil
}

// func GetRayJobResponseFromS3(rayJobID string) (string, error) {
// 	rayServiceURL := GetRayJobApiHost() + "/api/jobs/" + rayJobID + "/response"
// 	req, err := http.NewRequest("GET", rayServiceURL, nil)
// 	if err != nil {
// 		return "", err
// 	}
// 	req.Header.Set("Content-Type", "application/json")

// 	// Send the request to the Ray service
// 	client := GetRayClient()
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer resp.Body.Close()
// 	return resp.Status, nil
// }

func JobIsRunning(rayJobID string) bool {
	status, err := GetRayJobStatus(rayJobID)
	if err != nil {
		return false
	}
	return status == "RUNNING"
}

func JobIsPending(rayJobID string) bool {
	status, err := GetRayJobStatus(rayJobID)
	if err != nil {
		return false
	}
	return status == "PENDING"
}

func JobSucceeded(rayJobID string) bool {
	status, err := GetRayJobStatus(rayJobID)
	if err != nil {
		return false
	}
	return status == "SUCCEEDED"
}

func JobFailed(rayJobID string) bool {
	status, err := GetRayJobStatus(rayJobID)
	if err != nil {
		return false
	}
	return status == "FAILED"
}

func JobStopped(rayJobID string) bool {
	status, err := GetRayJobStatus(rayJobID)
	if err != nil {
		return false
	}
	return status == "STOPPED"
}

func validateInputKeys(inputVectors map[string]interface{}, modelInputs map[string]ipwl.ModelInput) error {
	for inputKey := range inputVectors {
		if _, exists := modelInputs[inputKey]; !exists {
			log.Printf("The argument %s is not in the model inputs.\n", inputKey)
			log.Printf("Available keys: %v\n", modelInputs)
			return fmt.Errorf("the argument %s is not in the model inputs", inputKey)
		}
	}
	return nil
}

func SubmitRayJob(job models.Job, modelPath string, rayJobID string, inputs map[string]interface{}, db *gorm.DB) (*http.Response, error) {
	log.Printf("Creating Ray job with modelPath: %s and inputs: %+v\n", modelPath, inputs)
	resp, err := CreateRayJob(&job, modelPath, rayJobID, inputs, db)
	if err != nil {
		log.Printf("Error creating Ray job: %v\n", err)
		return nil, err
	}

	log.Printf("Ray job finished with response status: %s\n", resp.Status)
	return resp, nil
}
