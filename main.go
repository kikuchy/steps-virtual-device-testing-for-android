package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-tools/go-steputils/input"
	"github.com/bitrise-tools/go-steputils/tools"
)

// ConfigsModel ...
type ConfigsModel struct {
	// api
	APIBaseURL string
	BuildSlug  string
	AppSlug    string
	APIToken   string

	// shared
	ApkPath              string
	TestApkPath          string
	TestType             string
	TestDevices          string
	AppPackageID         string
	TestTimeout          string
	DownloadTestResults  string
	DirectoriesToPull    string
	EnvironmentVariables string

	// instrumentation
	InstTestPackageID   string
	InstTestRunnerClass string
	InstTestTargets     string

	// robo
	RoboInitialActivity string
	RoboMaxDepth        string
	RoboMaxSteps        string
	RoboDirectives      string

	// loop
	LoopScenarios      string
	LoopScenarioLabels string
}

// ListStepsResponse ...
type ListStepsResponse struct {
	Steps []*Step `json:"steps,omitempty"`
}

// Outcome ...
type Outcome struct {
	FailureDetail      *FailureDetail      `json:"failureDetail,omitempty"`
	InconclusiveDetail *InconclusiveDetail `json:"inconclusiveDetail,omitempty"`
	SkippedDetail      *SkippedDetail      `json:"skippedDetail,omitempty"`
	SuccessDetail      *SuccessDetail      `json:"successDetail,omitempty"`
	Summary            string              `json:"summary,omitempty"`
}

// SuccessDetail ...
type SuccessDetail struct {
	OtherNativeCrash bool `json:"otherNativeCrash,omitempty"`
}

// SkippedDetail ...
type SkippedDetail struct {
	IncompatibleAppVersion   bool `json:"incompatibleAppVersion,omitempty"`
	IncompatibleArchitecture bool `json:"incompatibleArchitecture,omitempty"`
	IncompatibleDevice       bool `json:"incompatibleDevice,omitempty"`
}

// FailureDetail ...
type FailureDetail struct {
	Crashed          bool `json:"crashed,omitempty"`
	NotInstalled     bool `json:"notInstalled,omitempty"`
	OtherNativeCrash bool `json:"otherNativeCrash,omitempty"`
	TimedOut         bool `json:"timedOut,omitempty"`
	UnableToCrawl    bool `json:"unableToCrawl,omitempty"`
}

// InconclusiveDetail ...
type InconclusiveDetail struct {
	AbortedByUser         bool `json:"abortedByUser,omitempty"`
	InfrastructureFailure bool `json:"infrastructureFailure,omitempty"`
}

// Step ...
type Step struct {
	Outcome        *Outcome                   `json:"outcome,omitempty"`
	State          string                     `json:"state,omitempty"`
	DimensionValue []*StepDimensionValueEntry `json:"dimensionValue,omitempty"`
}

// StepDimensionValueEntry ...
type StepDimensionValueEntry struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// AndroidDevice ...
type AndroidDevice struct {
	AndroidModelID   string `json:"androidModelId,omitempty"`
	AndroidVersionID string `json:"androidVersionId,omitempty"`
	Locale           string `json:"locale,omitempty"`
	Orientation      string `json:"orientation,omitempty"`
}

// AndroidDeviceList ...
type AndroidDeviceList struct {
	AndroidDevices []*AndroidDevice `json:"androidDevices,omitempty"`
}

// EnvironmentMatrix ...
type EnvironmentMatrix struct {
	AndroidDeviceList *AndroidDeviceList `json:"androidDeviceList,omitempty"`
}

// TestMatrix ...
type TestMatrix struct {
	EnvironmentMatrix *EnvironmentMatrix `json:"environmentMatrix,omitempty"`
	TestSpecification *TestSpecification `json:"testSpecification,omitempty"`
}

// TestSpecification ...
type TestSpecification struct {
	AndroidInstrumentationTest *AndroidInstrumentationTest `json:"androidInstrumentationTest,omitempty"`
	AndroidRoboTest            *AndroidRoboTest            `json:"androidRoboTest,omitempty"`
	AndroidTestLoop            *AndroidTestLoop            `json:"androidTestLoop,omitempty"`
	AutoGoogleLogin            bool                        `json:"autoGoogleLogin,omitempty"`
	TestSetup                  *TestSetup                  `json:"testSetup,omitempty"`
	TestTimeout                string                      `json:"testTimeout,omitempty"`
}

// AndroidInstrumentationTest ...
type AndroidInstrumentationTest struct {
	AppPackageID    string   `json:"appPackageId,omitempty"`
	TestPackageID   string   `json:"testPackageId,omitempty"`
	TestRunnerClass string   `json:"testRunnerClass,omitempty"`
	TestTargets     []string `json:"testTargets,omitempty"`
}

// AndroidRoboTest ...
type AndroidRoboTest struct {
	AppInitialActivity string           `json:"appInitialActivity,omitempty"`
	AppPackageID       string           `json:"appPackageId,omitempty"`
	MaxDepth           int64            `json:"maxDepth,omitempty"`
	MaxSteps           int64            `json:"maxSteps,omitempty"`
	RoboDirectives     []*RoboDirective `json:"roboDirectives,omitempty"`
}

// RoboDirective ...
type RoboDirective struct {
	ActionType   string `json:"actionType,omitempty"`
	InputText    string `json:"inputText,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
}

// AndroidTestLoop ...
type AndroidTestLoop struct {
	AppPackageID   string   `json:"appPackageId,omitempty"`
	ScenarioLabels []string `json:"scenarioLabels,omitempty"`
	Scenarios      []int64  `json:"scenarios,omitempty"`
}

// TestSetup ...
type TestSetup struct {
	DirectoriesToPull    []string               `json:"directoriesToPull,omitempty"`
	EnvironmentVariables []*EnvironmentVariable `json:"environmentVariables,omitempty"`
	NetworkProfile       string                 `json:"networkProfile,omitempty"`
}

// EnvironmentVariable ...
type EnvironmentVariable struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// UploadURLRequest ...
type UploadURLRequest struct {
	AppURL     string `json:"appUrl"`
	TestAppURL string `json:"testAppUrl"`
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		// api
		APIBaseURL: os.Getenv("api_base_url"),
		BuildSlug:  os.Getenv("BITRISE_BUILD_SLUG"),
		AppSlug:    os.Getenv("BITRISE_APP_SLUG"),
		APIToken:   os.Getenv("api_token"),

		// shared
		ApkPath:              os.Getenv("apk_path"),
		TestApkPath:          os.Getenv("test_apk_path"),
		TestType:             os.Getenv("test_type"),
		TestDevices:          os.Getenv("test_devices"),
		AppPackageID:         os.Getenv("app_package_id"),
		TestTimeout:          os.Getenv("test_timeout"),
		DownloadTestResults:  os.Getenv("download_test_results"),
		DirectoriesToPull:    os.Getenv("directories_to_pull"),
		EnvironmentVariables: os.Getenv("environment_variables"),

		// instrumentation
		InstTestPackageID:   os.Getenv("inst_test_package_id"),
		InstTestRunnerClass: os.Getenv("inst_test_runner_class"),
		InstTestTargets:     os.Getenv("inst_test_targets"),

		// robo
		RoboInitialActivity: os.Getenv("robo_initial_activity"),
		RoboMaxDepth:        os.Getenv("robo_max_depth"),
		RoboMaxSteps:        os.Getenv("robo_max_steps"),
		RoboDirectives:      os.Getenv("robo_directives"),

		// loop
		LoopScenarios:      os.Getenv("loop_scenarios"),
		LoopScenarioLabels: os.Getenv("loop_scenario_labels"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Configs:")
	log.Printf("- ApkPath: %s", configs.ApkPath)

	log.Printf("- TestTimeout: %s", configs.TestTimeout)
	log.Printf("- DirectoriesToPull: %s", configs.DirectoriesToPull)
	log.Printf("- EnvironmentVariables: %s", configs.EnvironmentVariables)
	log.Printf("- TestDevices:\n---")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "Model\tAPI Level\tLocale\tOrientation\t")
	scanner := bufio.NewScanner(strings.NewReader(configs.TestDevices))
	for scanner.Scan() {
		device := scanner.Text()
		device = strings.TrimSpace(device)
		if device == "" {
			continue
		}

		deviceParams := strings.Split(device, ",")

		if len(deviceParams) != 4 {
			continue
		}

		fmt.Fprintln(w, fmt.Sprintf("%s\t%s\t%s\t%s\t", deviceParams[0], deviceParams[1], deviceParams[3], deviceParams[2]))
	}
	if err := w.Flush(); err != nil {
		log.Errorf("Failed to flush writer, error: %s", err)
	}
	log.Printf("---")
	log.Printf("- AppPackageID: %s", configs.AppPackageID)
	log.Printf("- TestType: %s", configs.TestType)

	// instruments
	if configs.TestType == "instrumentation" {
		log.Printf("- TestApkPath: %s", configs.TestApkPath)
		log.Printf("- InstTestPackageID: %s", configs.InstTestPackageID)
		log.Printf("- InstTestRunnerClass: %s", configs.InstTestRunnerClass)
		log.Printf("- InstTestTargets: %s", configs.InstTestTargets)
	}

	//robo
	if configs.TestType == "robo" {
		log.Printf("- RoboInitialActivity: %s", configs.RoboInitialActivity)
		log.Printf("- RoboMaxDepth: %s", configs.RoboMaxDepth)
		log.Printf("- RoboMaxSteps: %s", configs.RoboMaxSteps)
		log.Printf("- RoboDirectives: %s", configs.RoboDirectives)
	}

	if configs.TestType == "gameloop" {
		// loop
		log.Printf("- LoopScenarios: %s", configs.LoopScenarios)
		log.Printf("- LoopScenarioLabels: %s", configs.LoopScenarioLabels)
	}
}

func (configs ConfigsModel) validate() error {

	if err := input.ValidateIfNotEmpty(configs.APIBaseURL); err != nil {
		return fmt.Errorf("Issue with APIBaseURL: %s", err)
	}
	if err := input.ValidateIfNotEmpty(configs.APIToken); err != nil {
		return fmt.Errorf("Issue with APIToken: %s", err)
	}
	if err := input.ValidateIfNotEmpty(configs.BuildSlug); err != nil {
		return fmt.Errorf("Issue with BuildSlug: %s", err)
	}
	if err := input.ValidateIfNotEmpty(configs.AppSlug); err != nil {
		return fmt.Errorf("Issue with AppSlug: %s", err)
	}
	if err := input.ValidateIfNotEmpty(configs.TestType); err != nil {
		return fmt.Errorf("Issue with TestType: %s", err)
	}
	if err := input.ValidateWithOptions(configs.TestType, "instrumentation", "robo", "gameloop"); err != nil {
		return fmt.Errorf("Issue with TestType: %s", err)
	}
	if err := input.ValidateIfNotEmpty(configs.ApkPath); err != nil {
		return fmt.Errorf("Issue with ApkPath: %s", err)
	}
	if err := input.ValidateIfPathExists(configs.ApkPath); err != nil {
		return fmt.Errorf("Issue with ApkPath: %s", err)
	}
	if configs.TestType == "instrumentation" {
		if err := input.ValidateIfNotEmpty(configs.TestApkPath); err != nil {
			return fmt.Errorf("Issue with TestApkPath: %s", err)
		}
		if err := input.ValidateIfPathExists(configs.TestApkPath); err != nil {
			return fmt.Errorf("Issue with TestApkPath: %s", err)
		}
	}

	return nil
}

func failf(f string, v ...interface{}) {
	log.Errorf(f, v)
	os.Exit(1)
}

func main() {
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		failf("%s", err)
	}

	fmt.Println()

	successful := true

	log.Infof("Upload APKs")
	{
		url := configs.APIBaseURL + "/assets/" + configs.AppSlug + "/" + configs.BuildSlug + "/" + configs.APIToken

		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			failf("Failed to create http request, error: %s", err)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			failf("Failed to get http response, error: %s", err)
		}

		if resp.StatusCode != http.StatusOK {
			failf("Failed to get http response, status code: %d", resp.StatusCode)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			failf("Failed to read response body, error: %s", err)
		}

		responseModel := &UploadURLRequest{}

		err = json.Unmarshal(body, responseModel)
		if err != nil {
			failf("Failed to unmarshal response body, error: %s", err)
		}

		err = uploadFile(responseModel.AppURL, configs.ApkPath)
		if err != nil {
			failf("Failed to upload file(%s) to (%s), error: %s", configs.ApkPath, responseModel.AppURL, err)
		}

		if configs.TestType == "instrumentation" {
			err = uploadFile(responseModel.TestAppURL, configs.TestApkPath)
			if err != nil {
				failf("Failed to upload file(%s) to (%s), error: %s", configs.TestApkPath, responseModel.TestAppURL, err)
			}
		}

		log.Donef("=> APKs uploaded")
	}

	fmt.Println()
	log.Infof("Start test")
	{
		url := configs.APIBaseURL + "/" + configs.AppSlug + "/" + configs.BuildSlug + "/" + configs.APIToken

		testModel := &TestMatrix{}
		testModel.EnvironmentMatrix = &EnvironmentMatrix{AndroidDeviceList: &AndroidDeviceList{}}
		testModel.EnvironmentMatrix.AndroidDeviceList.AndroidDevices = []*AndroidDevice{}

		scanner := bufio.NewScanner(strings.NewReader(configs.TestDevices))
		for scanner.Scan() {
			device := scanner.Text()
			device = strings.TrimSpace(device)
			if device == "" {
				continue
			}

			deviceParams := strings.Split(device, ",")
			if len(deviceParams) != 4 {
				failf("Invalid test device configuration: %s", device)
			}

			newDevice := AndroidDevice{
				AndroidModelID:   deviceParams[0],
				AndroidVersionID: deviceParams[1],
				Locale:           deviceParams[2],
				Orientation:      deviceParams[3],
			}

			testModel.EnvironmentMatrix.AndroidDeviceList.AndroidDevices = append(testModel.EnvironmentMatrix.AndroidDeviceList.AndroidDevices, &newDevice)
		}

		// parse directories to pull
		scanner = bufio.NewScanner(strings.NewReader(configs.DirectoriesToPull))
		directoriesToPull := []string{}
		for scanner.Scan() {
			path := scanner.Text()
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			directoriesToPull = append(directoriesToPull, path)
		}

		// parse environment variables
		scanner = bufio.NewScanner(strings.NewReader(configs.DirectoriesToPull))
		envs := []*EnvironmentVariable{}
		for scanner.Scan() {
			envStr := scanner.Text()

			if envStr == "" {
				continue
			}

			if !strings.Contains(envStr, "=") {
				continue
			}

			envStrSplit := strings.Split(envStr, "=")
			envKey := envStrSplit[0]
			envValue := strings.Join(envStrSplit[1:], "=")

			envs = append(envs, &EnvironmentVariable{Key: envKey, Value: envValue})
		}

		testModel.TestSpecification = &TestSpecification{
			TestTimeout: fmt.Sprintf("%ss", configs.TestTimeout),
			TestSetup: &TestSetup{
				EnvironmentVariables: envs,
				DirectoriesToPull:    directoriesToPull,
			},
		}

		switch configs.TestType {
		case "instrumentation":
			testModel.TestSpecification.AndroidInstrumentationTest = &AndroidInstrumentationTest{}
			if configs.AppPackageID != "" {
				testModel.TestSpecification.AndroidInstrumentationTest.AppPackageID = configs.AppPackageID
			}
			if configs.InstTestPackageID != "" {
				testModel.TestSpecification.AndroidInstrumentationTest.TestPackageID = configs.InstTestPackageID
			}
			if configs.InstTestRunnerClass != "" {
				testModel.TestSpecification.AndroidInstrumentationTest.TestRunnerClass = configs.InstTestRunnerClass
			}
			if configs.InstTestTargets != "" {
				targets := strings.Split(strings.TrimSpace(configs.InstTestTargets), ",")
				testModel.TestSpecification.AndroidInstrumentationTest.TestTargets = targets
			}
		case "robo":
			testModel.TestSpecification.AndroidRoboTest = &AndroidRoboTest{}
			if configs.AppPackageID != "" {
				testModel.TestSpecification.AndroidRoboTest.AppPackageID = configs.AppPackageID
			}
			if configs.RoboInitialActivity != "" {
				testModel.TestSpecification.AndroidRoboTest.AppInitialActivity = configs.RoboInitialActivity
			}
			if configs.RoboMaxDepth != "" {
				maxDepth, err := strconv.Atoi(configs.RoboMaxDepth)
				if err != nil {
					failf("Failed to parse string(%s) to integer, error: %s", configs.RoboMaxDepth, err)
				}
				testModel.TestSpecification.AndroidRoboTest.MaxDepth = int64(maxDepth)
			}
			if configs.RoboMaxSteps != "" {
				maxSteps, err := strconv.Atoi(configs.RoboMaxSteps)
				if err != nil {
					failf("Failed to parse string(%s) to integer, error: %s", configs.RoboMaxSteps, err)
				}
				testModel.TestSpecification.AndroidRoboTest.MaxSteps = int64(maxSteps)
			}
			if configs.RoboDirectives != "" {
				roboDirectives := []*RoboDirective{}
				scanner := bufio.NewScanner(strings.NewReader(configs.RoboDirectives))
				for scanner.Scan() {
					directive := scanner.Text()
					directive = strings.TrimSpace(directive)
					if directive == "" {
						continue
					}

					directiveParams := strings.Split(directive, ",")
					if len(directiveParams) != 3 {
						failf("Invalid directive configuration: %s", directive)
					}
					roboDirectives = append(roboDirectives, &RoboDirective{ResourceName: directiveParams[0], InputText: directiveParams[1], ActionType: directiveParams[2]})
				}
				testModel.TestSpecification.AndroidRoboTest.RoboDirectives = roboDirectives
			}
		case "gameloop":
			testModel.TestSpecification.AndroidTestLoop = &AndroidTestLoop{}
			if configs.AppPackageID != "" {
				testModel.TestSpecification.AndroidTestLoop.AppPackageID = configs.AppPackageID
			}
			if configs.LoopScenarios != "" {
				loopScenarios := []int64{}
				for _, scenarioStr := range strings.Split(strings.TrimSpace(configs.LoopScenarios), ",") {
					scenario, err := strconv.Atoi(scenarioStr)
					if err != nil {
						failf("Failed to parse string(%s) to integer, error: %s", scenarioStr, err)
					}
					loopScenarios = append(loopScenarios, int64(scenario))
				}
				testModel.TestSpecification.AndroidTestLoop.Scenarios = loopScenarios
			}
			if configs.LoopScenarioLabels != "" {
				scenarioLabels := strings.Split(strings.TrimSpace(configs.LoopScenarioLabels), ",")
				testModel.TestSpecification.AndroidTestLoop.ScenarioLabels = scenarioLabels
			}
		}

		jsonByte, err := json.Marshal(testModel)
		if err != nil {
			failf("Failed to marshal test model, error: %s", err)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonByte))
		if err != nil {
			failf("Failed to create http request, error: %s", err)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			failf("Failed to get http response, error: %s", err)
		}

		if resp.StatusCode != http.StatusOK {
			failf("Failed to get http response, status code: %d", resp.StatusCode)
		}

		log.Donef("=> Test started")
	}

	fmt.Println()
	log.Infof("Waiting for test results")
	{
		finished := false
		printedLogs := []string{}
		for !finished {
			url := configs.APIBaseURL + "/" + configs.AppSlug + "/" + configs.BuildSlug + "/" + configs.APIToken

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				failf("Failed to create http request, error: %s", err)
			}

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				failf("Failed to get http response, error: %s", err)
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				failf("Failed to read response body, error: %s", err)
			}

			responseModel := &ListStepsResponse{}

			err = json.Unmarshal(body, responseModel)
			if err != nil {
				failf("Failed to unmarshal response body, error: %s, body: %s", err, string(body))
			}

			finished = true
			testsRunning := 0
			for _, step := range responseModel.Steps {
				if step.State != "complete" {
					finished = false
					testsRunning++
				}
			}

			msg := ""
			if len(responseModel.Steps) == 0 {
				finished = false
				msg = fmt.Sprintf("- Validating")
			} else {
				msg = fmt.Sprintf("- (%d/%d) running", testsRunning, len(responseModel.Steps))
			}

			if !sliceutil.IsStringInSlice(msg, printedLogs) {
				log.Printf(msg)
				printedLogs = append(printedLogs, msg)
			}

			if finished {
				log.Donef("=> Test finished")
				fmt.Println()

				log.Infof("Test results:")
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "Model\tAPI Level\tLocale\tOrientation\tOutcome\t")

				for _, step := range responseModel.Steps {
					dimensions := map[string]string{}
					for _, dimension := range step.DimensionValue {
						dimensions[dimension.Key] = dimension.Value
					}

					outcome := step.Outcome.Summary

					switch outcome {
					case "success":
						outcome = colorstring.Green(outcome)
					case "failure":
						successful = false
						if step.Outcome.FailureDetail != nil {
							if step.Outcome.FailureDetail.Crashed {
								outcome += "(Crashed)"
							}
							if step.Outcome.FailureDetail.NotInstalled {
								outcome += "(NotInstalled)"
							}
							if step.Outcome.FailureDetail.OtherNativeCrash {
								outcome += "(OtherNativeCrash)"
							}
							if step.Outcome.FailureDetail.TimedOut {
								outcome += "(TimedOut)"
							}
							if step.Outcome.FailureDetail.UnableToCrawl {
								outcome += "(UnableToCrawl)"
							}
						}
						outcome = colorstring.Red(outcome)
					case "inconclusive":
						successful = false
						if step.Outcome.InconclusiveDetail != nil {
							if step.Outcome.InconclusiveDetail.AbortedByUser {
								outcome += "(AbortedByUser)"
							}
							if step.Outcome.InconclusiveDetail.InfrastructureFailure {
								outcome += "(InfrastructureFailure)"
							}
						}
						outcome = colorstring.Yellow(outcome)
					case "skipped":
						successful = false
						if step.Outcome.SkippedDetail != nil {
							if step.Outcome.SkippedDetail.IncompatibleAppVersion {
								outcome += "(IncompatibleAppVersion)"
							}
							if step.Outcome.SkippedDetail.IncompatibleArchitecture {
								outcome += "(IncompatibleArchitecture)"
							}
							if step.Outcome.SkippedDetail.IncompatibleDevice {
								outcome += "(IncompatibleDevice)"
							}
						}
						outcome = colorstring.Blue(outcome)
					}

					fmt.Fprintln(w, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t", dimensions["Model"], dimensions["Version"], dimensions["Locale"], dimensions["Orientation"], outcome))
				}
				if err := w.Flush(); err != nil {
					log.Errorf("Failed to flush writer, error: %s", err)
				}
			}
			if !finished {
				time.Sleep(5 * time.Second)
			}
		}
	}

	if configs.DownloadTestResults == "true" {
		fmt.Println()
		log.Infof("Downloading test assets")
		{
			url := configs.APIBaseURL + "/assets/" + configs.AppSlug + "/" + configs.BuildSlug + "/" + configs.APIToken

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				failf("Failed to create http request, error: %s", err)
			}

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				failf("Failed to get http response, error: %s", err)
			}

			if resp.StatusCode != http.StatusOK {
				failf("Failed to get http response, status code: %d", resp.StatusCode)
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				failf("Failed to read response body, error: %s", err)
			}

			responseModel := map[string]string{}

			err = json.Unmarshal(body, &responseModel)
			if err != nil {
				failf("Failed to unmarshal response body, error: %s", err)
			}

			tempDir, err := pathutil.NormalizedOSTempDirPath("vdtesting_test_assets")
			if err != nil {
				failf("Failed to create temp dir, error: %s", err)
			}

			for fileName, fileURL := range responseModel {
				err := downloadFile(fileURL, filepath.Join(tempDir, fileName))
				if err != nil {
					failf("Failed to download file, error: %s", err)
				}
			}

			log.Donef("=> Assets downloaded")
			if err := tools.ExportEnvironmentWithEnvman("VDTESTING_DOWNLOADED_FILES_DIR", tempDir); err != nil {
				log.Warnf("Failed to export environment (VDTESTING_DOWNLOADED_FILES_DIR), error: %s", err)
			} else {
				log.Printf("The downloaded test assets path (%s) is exported to the VDTESTING_DOWNLOADED_FILES_DIR environment variable.", tempDir)
			}
		}
	}

	if !successful {
		os.Exit(1)
	}
}

func downloadFile(url string, localPath string) error {
	out, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("Failed to open the local cache file for write: %s", err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Printf("Failed to close Archive download file (%s): %s", localPath, err)
		}
	}()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to create cache download request: %s", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close Archive download response body: %s", err)
		}
	}()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Failed to download archive - non success response code: %d", resp.StatusCode)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to save cache content into file: %s", err)
	}

	return nil
}

func uploadFile(uploadURL string, archiveFilePath string) error {
	archFile, err := os.Open(archiveFilePath)
	if err != nil {
		return fmt.Errorf("Failed to open archive file for upload (%s): %s", archiveFilePath, err)
	}
	isFileCloseRequired := true
	defer func() {
		if !isFileCloseRequired {
			return
		}
		if err := archFile.Close(); err != nil {
			log.Printf(" (!) Failed to close archive file (%s): %s", archiveFilePath, err)
		}
	}()

	fileInfo, err := archFile.Stat()
	if err != nil {
		return fmt.Errorf("Failed to get File Stats of the Archive file (%s): %s", archiveFilePath, err)
	}
	fileSize := fileInfo.Size()

	req, err := http.NewRequest("PUT", uploadURL, archFile)
	if err != nil {
		return fmt.Errorf("Failed to create upload request: %s", err)
	}

	req.Header.Add("Content-Length", strconv.FormatInt(fileSize, 10))
	req.ContentLength = fileSize

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to upload: %s", err)
	}
	isFileCloseRequired = false
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf(" [!] Failed to close response body: %s", err)
		}
	}()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response: %s", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("Failed to upload file, response code was: %d", resp.StatusCode)
	}

	return nil
}
