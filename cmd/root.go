/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "circleci-cancel-redundant-cli",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		err := doCancelRedundantWorkflow()
		if err != nil {
			fmt.Printf("Run failed: %+v\n", err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.circleci-cancel-redundant-cli.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

type client struct {
	token       string
	basePath    *url.URL
	projectSlug string
}

func request[Response any](client *client, p string, method string, body io.Reader) (Response, error) {
	u := client.basePath
	u.Path = path.Join(u.Path, p)
	fullPath := u.String()
	req, err := http.NewRequest(method, fullPath, body)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	req.Header.Add("Circle-Token", "token")

	log.Printf("do request: %s", req.URL)
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	var d Response
	err = json.Unmarshal(b, &d)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	return d, nil
}

type GetWorkflowResponse struct {
	PipelineId     string    `json:"pipeline_id"`
	CanceledBy     string    `json:"canceled_by"`
	Id             string    `json:"id"`
	Name           string    `json:"name"`
	ProjectSlug    string    `json:"project_slug"`
	ErroredBy      string    `json:"errored_by"`
	Tag            string    `json:"tag"`
	Status         string    `json:"status"`
	StartedBy      string    `json:"started_by"`
	PipelineNumber int       `json:"pipeline_number"`
	CreatedAt      time.Time `json:"created_at"`
	StoppedAt      time.Time `json:"stopped_at"`
}

type GetProjectPipelineResponse struct {
	Items []struct {
		Id     string `json:"id"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
		ProjectSlug string    `json:"project_slug"`
		UpdatedAt   time.Time `json:"updated_at"`
		Number      int       `json:"number"`
		State       string    `json:"state"`
		CreatedAt   time.Time `json:"created_at"`
		Trigger     struct {
			Type       string    `json:"type"`
			ReceivedAt time.Time `json:"received_at"`
			Actor      struct {
				Login     string `json:"login"`
				AvatarUrl string `json:"avatar_url"`
			} `json:"actor"`
		} `json:"trigger"`
		Vcs struct {
			ProviderName        string `json:"provider_name"`
			TargetRepositoryUrl string `json:"target_repository_url"`
			Branch              string `json:"branch"`
			ReviewId            string `json:"review_id"`
			ReviewUrl           string `json:"review_url"`
			Revision            string `json:"revision"`
			Tag                 string `json:"tag"`
			Commit              struct {
				Subject string `json:"subject"`
				Body    string `json:"body"`
			} `json:"commit"`
			OriginRepositoryUrl string `json:"origin_repository_url"`
		} `json:"vcs"`
	} `json:"items"`
	NextPageToken string `json:"next_page_token"`
}

type GetPipelineWorkflowResponse struct {
	Items []struct {
		PipelineId     string    `json:"pipeline_id"`
		CanceledBy     string    `json:"canceled_by"`
		Id             string    `json:"id"`
		Name           string    `json:"name"`
		ProjectSlug    string    `json:"project_slug"`
		ErroredBy      string    `json:"errored_by"`
		Tag            string    `json:"tag"`
		Status         string    `json:"status"`
		StartedBy      string    `json:"started_by"`
		PipelineNumber int       `json:"pipeline_number"`
		CreatedAt      time.Time `json:"created_at"`
		StoppedAt      time.Time `json:"stopped_at"`
	} `json:"items"`
	NextPageToken string `json:"next_page_token"`
}

type PostCancelResponse struct {
	Message string `json:"message"`
}

func doCancelRedundantWorkflow() error {
	orgSlug, err := circleProjectUsername()
	if err != nil {
		return fmt.Errorf("doCancelRedundantWorkflow: get project org slug failed: %w", err)
	}
	vcsSlug := "gh"

	repoName, err := circleProjectReponame()
	if err != nil {
		return fmt.Errorf("doCancelRedundantWorkflow: get project reponame failed: %w", err)
	}

	workflowId, err := circleWorkflowId()
	if err != nil {
		return fmt.Errorf("doCancelRedundantWorkflow: get workflowId failed: %w", err)
	}

	token, err := circleToken()
	if err != nil {
		return fmt.Errorf("doCancelRedundantWorkflow: get token failed: %w", err)
	}

	baseUrl, err := url.Parse("https://circleci.com/api/v2/")
	if err != nil {
		return fmt.Errorf("doCancelRedundantWorkflow: parse baseUrl failed: %w", err)
	}

	c := &client{
		token:       token,
		basePath:    baseUrl,
		projectSlug: fmt.Sprintf("%s/%s/%s", vcsSlug, orgSlug, repoName),
	}

	currentWorkflow, err := request[GetWorkflowResponse](c, fmt.Sprintf("workflows/%s", workflowId), "GET", nil)
	if err != nil {
		return fmt.Errorf("doCancelRedundantWorkflow: get workflow request failed: %w", err)
	}

	branchName, err := circleBranch()
	if err != nil {
		return fmt.Errorf("doCancelRedundantWorkflow: get branch name failed: %w", err)
	}

	pipes, err := request[GetProjectPipelineResponse](c, fmt.Sprintf("project/project/%s/%s/%s/pipeline?branch=%s", vcsSlug, orgSlug, repoName, branchName), "GET", nil)
	if err != nil {
		return fmt.Errorf("doCancelRedundantWorkflow: get pipelines failed: %w", err)
	}

	items := pipes.Items

	for _, item := range items {
		if item.State != "created" {
			continue
		}
		pipeId := item.Id
		workflows, err := request[GetPipelineWorkflowResponse](c, fmt.Sprintf("pipeline/%s/workflow", pipeId), "GET", nil)
		if err != nil {
			return fmt.Errorf("doCancelRedundantWorkflow: get workflows failed: %w", err)
		}
		for _, workflow := range workflows.Items {
			if (workflow.Status == "on_hold" || workflow.Status == "running") && workflow.Name == currentWorkflow.Name && workflow.Id != workflowId {
				cancel, err := request[PostCancelResponse](c, fmt.Sprintf("workflow/%s/cancel", workflow.Id), "POST", nil)
				if err != nil {
					return fmt.Errorf("doCancelRedundantWorkflow: cancel workflow failed: %w", err)
				}
				fmt.Printf("cancel success: id: %s, message: %s", workflow.Id, cancel.Message)
			}
		}
	}
	fmt.Println("finish cancel")
	return nil
}

func circleProjectUsername() (string, error) {
	return orError("CIRCLE_PROJECT_USERNAME")
}

func circleProjectReponame() (string, error) {
	return orError("CIRCLE_PROJECT_REPONAME")
}

func circleBranch() (string, error) {
	return orError("CIRCLE_BRANCH")
}

func circleUserName() (string, error) {
	return orError("CIRCLE_USERNAME")
}

func circleWorkflowId() (string, error) {
	return orError("CIRCLE_WORKFLOW_ID")
}

func circleToken() (string, error) {
	return orError("CIRCLE_TOKEN")
}

func orError(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", errors.New(fmt.Sprintf("orError: required env var %s is not provided", key))
	}
	return val, nil
}
