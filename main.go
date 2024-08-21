// escargo is a write-back solution for argocd
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/xanzy/go-gitlab"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "escargo",
	Short: "escargo - a simple CLI to modify helm value paths for argocd write-back",
	Long: `escargo is what they call 'superdope'. Its like argocd-image-updater, but slimier! 
      
escargo expects to be run from the root of a repository, and accepts named inputs to write specific values to specific value paths in a given repository`,
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s' \n\n", err)
		os.Exit(1)
	}
}

var dryRun bool
var filePath, gitlabToken, newValue, valuePath string

func init() {
	// set flags
	// pass these values to escargo at runtime
	flag.BoolVarP(&dryRun, "dryRun", "d", true, "dryRun")
	flag.StringVarP(&filePath, "filePath", "f", "", "path to helm values file")
	flag.StringVarP(&gitlabToken, "gitlabToken", "g", "", "gitlab access token")
	flag.StringVarP(&newValue, "newValue", "n", "", "value to write at the valuePath within the given filePath")
	flag.StringVarP(&valuePath, "valuePath", "p", "", "helm values path key to write to within the given filePath")
	flag.Parse()

	// there is a more elegant way to do this with cobra
	if filePath == "" || gitlabToken == "" || newValue == "" || valuePath == "" {
		fmt.Printf("error: all flags are required, see help for more info")
		os.Exit(1)
	}
}

func writeGit() {
	// assume the file has already been updated by viperUpdateFile

	// your gitlab projectId
	projectId := "00000000"

	// init gitlab client
	git, err := gitlab.NewClient(gitlabToken)
	if err != nil {
		log.Fatalf("Failed to create client: %v \n\n", err)
	}

	// if dryRun, then don't make changes
	if dryRun {
		fmt.Printf("dryRun enabled, bypassing gitlab writes")
	} else {
		// where are we merging to?
		targetBranch := "main"

		// commit and merge message
		commitMessage := "updating " + filePath + " at value path " + valuePath + " with value: " + newValue

		// set up commit and branch
		branchName := "escargo/" + newValue
		createBranchOptions := &gitlab.CreateBranchOptions{
			Branch: gitlab.String(branchName),
			Ref:    gitlab.String("main"),
		}

		// check if branch exists
		listBranchOptions := &gitlab.ListBranchesOptions{
			Search: gitlab.String(*createBranchOptions.Branch),
		}
		listBranches, _, err := git.Branches.ListBranches(projectId, listBranchOptions)
		if err != nil {
			log.Fatal(err)
		}
		if len(listBranches) > 0 {
			log.Printf("Duplicate branch found, deleting: %s \n", listBranches[0])
			_, err := git.Branches.DeleteBranch(projectId, branchName, nil)
			if err != nil {
				log.Fatal(err)
			}
		}

		// create new branch
		newBranch, _, err := git.Branches.CreateBranch(projectId, createBranchOptions, nil)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Created Branch: %s \n", newBranch.Name)

		// commit change
		fileContents, err := os.ReadFile(filePath)
		fileText := string(fileContents)
		if err != nil {
			log.Fatal(err)
		}
		commitActionOptions := &gitlab.CommitActionOptions{
			Action:   gitlab.FileAction("update"),
			FilePath: gitlab.String(filePath),
			Content:  gitlab.String(fileText),
		}
		var commitActionOptionsList []*gitlab.CommitActionOptions = []*gitlab.CommitActionOptions{commitActionOptions}
		createCommitOptions := &gitlab.CreateCommitOptions{
			Branch:        gitlab.String(newBranch.Name),
			CommitMessage: gitlab.String(commitMessage),
			Actions:       commitActionOptionsList,
		}
		newCommit, _, err := git.Commits.CreateCommit(projectId, createCommitOptions, nil)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Created Commit: %s \n", newCommit.CommitterName)

		// check the commit diff
		getCommitDiffOptions := &gitlab.GetCommitDiffOptions{}
		getCommitDiff, _, err := git.Commits.GetCommitDiff(projectId, newBranch.Name, getCommitDiffOptions, nil)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Commit Diff: %s \n", getCommitDiff)
		if len(getCommitDiff) == 0 {
			fmt.Printf("Commit diff is empty, aborting and cleaning up branch!")
			_, err := git.Branches.DeleteBranch(projectId, branchName, nil)
			if err != nil {
				log.Fatal(err)
			}
			os.Exit(1)
		}

		// create MR
		createMergeRequestOptions := &gitlab.CreateMergeRequestOptions{
			Title:              gitlab.String(commitMessage),
			Description:        gitlab.String("automated merge request created by a slime!"),
			SourceBranch:       gitlab.String(newBranch.Name),
			TargetBranch:       gitlab.String(targetBranch),
			RemoveSourceBranch: gitlab.Bool(true),
		}

		newMergeRequest, _, err := git.MergeRequests.CreateMergeRequest(projectId, createMergeRequestOptions, nil)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Created Merge Request: %s \n", newMergeRequest.WebURL)

		// approve MR
		createMergeRequestApprovalOptions := &gitlab.ApproveMergeRequestOptions{}
		approveMergeRequest, _, err := git.MergeRequestApprovals.ApproveMergeRequest(projectId, newMergeRequest.IID, createMergeRequestApprovalOptions, nil)
		if err != nil {
			fmt.Printf("error approving MR: %s \n", err)
			log.Fatal(err)
		}
		log.Printf("Approved Merge Request: %s \n", approveMergeRequest.MergeStatus)

		// until MergeStatus is can_be_merged, check for merge status
		// this is due to latency on the gitlab side -- there is a delay between approving a merge request and the MR actually being mergeable
		for {
			status, _, err := git.MergeRequestApprovals.GetConfiguration(projectId, newMergeRequest.IID, nil)
			if err != nil {
				log.Fatal(err)
			}
			if status.MergeStatus != "can_be_merged" {
				fmt.Printf("warning: MergeStatus is: %s, retrying ...\n", status.MergeStatus)
				time.Sleep(1 * time.Second)
				if err != nil {
					log.Fatal(err)
				}
			} else {
				fmt.Printf("MergeStatus is: %s\n", status.MergeStatus)
				break
			}
		}

		// merge MR
		mergeOptions := &gitlab.AcceptMergeRequestOptions{
			MergeCommitMessage: gitlab.String(commitMessage),
		}
		mergeIt, _, err := git.MergeRequests.AcceptMergeRequest(projectId, newMergeRequest.IID, mergeOptions, nil)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("MR %d Success: Merged branch %s into %s!", mergeIt.ID, mergeIt.SourceBranch, mergeIt.TargetBranch)
	}
}

func yqUpdateFile() {
	// shell out to yq to perform the configuration change
	// NOTE: yq will reformat on write
	// this is a known issue with the upstream yaml lib, see https://github.com/mikefarah/yq/issues/465
	path, err := exec.LookPath("yq")
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command(path, "-i", "."+valuePath+" = \""+newValue+"\"", filePath)
	if dryRun {
		fmt.Printf("DRYRUN ENABLED, would run command: %s \n", cmd)
	} else {
		stdoutStderr, err := cmd.CombinedOutput()
		// run it
		fmt.Printf("Running command: %s \n", cmd)
		if err != nil {
			fmt.Printf("Output: %s \n", stdoutStderr)
		}
	}
}

func main() {
	// assume we're running in a gitlab-ci pipeline

	// run cobra handler
	Execute()

	// debug
	fmt.Fprintf(os.Stdout, "filePath: '%s', gitlabToken: '%s', newValue: '%s', valuePath: '%s' \n\n", filePath, gitlabToken, newValue, valuePath)

	// shell out to yq to perform the update
	yqUpdateFile()

	// init gitlab
	writeGit()
}
