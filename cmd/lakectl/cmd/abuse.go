package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/treeverse/lakefs/api/gen/models"

	"github.com/go-openapi/swag"

	"github.com/spf13/cobra"
	"github.com/treeverse/lakefs/cmdutils"
	"github.com/treeverse/lakefs/uri"
)

var abuseCmd = &cobra.Command{
	Use:    "abuse <sub command>",
	Short:  "abuse a running lakeFS instance. See sub commands for more info.",
	Hidden: true,
}

var abuseCreateBranchesCmd = &cobra.Command{
	Use:    "create-branches <source ref uri>",
	Short:  "Create a lot of branches very quickly.",
	Hidden: false,
	Args: cmdutils.ValidationChain(
		cobra.ExactArgs(1),
		cmdutils.FuncValidator(0, uri.ValidateRefURI),
	),
	Run: func(cmd *cobra.Command, args []string) {
		u := uri.Must(uri.Parse(args[0]))

		// only clean prefixed branches without creating new ones
		cleanOnly, err := cmd.Flags().GetBool("clean-only")
		if err != nil {
			DieErr(err)
		}

		// prefix to create new branches with
		branchPrefix, err := cmd.Flags().GetString("branch-prefix")
		if err != nil {
			DieErr(err)
		}
		client := getClient()

		// how many branches to create
		amount, err := cmd.Flags().GetInt("amount")
		if err != nil {
			DieErr(err)
		}

		// how many calls in parallel to execute
		parallelism, err := cmd.Flags().GetInt("parallelism")
		if err != nil {
			DieErr(err)
		}

		// delete all prefixed branches first for a clean start
		totalDeleted := 0
		semaphore := make(chan bool, parallelism)
		for {
			branches, pagination, err := client.ListBranches(context.Background(), u.Repository, branchPrefix, 1000)
			if err != nil {
				DieErr(err)
			}
			matches := 0
			var wg sync.WaitGroup
			for _, b := range branches {
				branch := swag.StringValue(b.ID)
				if !strings.HasPrefix(branch, branchPrefix) {
					continue
				}
				matches++
				totalDeleted++
				wg.Add(1)
				go func(branch string) {
					semaphore <- true
					err := client.DeleteBranch(context.Background(), u.Repository, branch)
					if err != nil {
						DieErr(err)
					}
					wg.Done()
					<-semaphore
				}(branch)
			}
			if matches == 0 {
				break // no more
			}
			wg.Wait() // wait for this batch to be over
			Fmt("branches deleted so far: %d\n", totalDeleted)
			if !swag.BoolValue(pagination.HasMore) {
				break
			}
		}
		Fmt("total deleted with prefix: %d\n", totalDeleted)

		if cleanOnly {
			return // done.
		}

		worker := func(ctx context.Context, inp chan string, out chan struct{}) {
			for {
				select {
				case x := <-inp:
					_, err := client.CreateBranch(ctx, u.Repository, &models.BranchCreation{
						Name:   &x,
						Source: &u.Ref,
					})
					if err != nil {
						DieErr(err)
					}
					out <- struct{}{}
				case <-ctx.Done():
					break
				}
			}
		}

		Fmt("creating %d branches now...\n", amount)
		reqs := make(chan string, amount)
		responses := make(chan struct{})
		for i := 1; i <= amount; i++ {
			reqs <- fmt.Sprintf("%s%d", branchPrefix, i)
		}

		// now run it with the given parallelism
		ctx := context.Background()
		for i := 0; i < parallelism; i++ {
			go worker(ctx, reqs, responses)
		}

		// collect responses
		i := 0
		t := time.Now()
		const printEvery = 10000
		start := time.Now()
		for {
			<-responses
			i++
			if i%printEvery == 0 {
				thisBatch := time.Since(t)
				Fmt("done %d calls in %s (%.2f/second)\n", i, thisBatch, float64(printEvery)/thisBatch.Seconds())
				t = time.Now()
			}
			if i == amount {
				break
			}
		}

		took := time.Since(start)
		Fmt("Done! created %d branches in %s: (%.2f/second)\n\n", amount, took, float64(amount)/took.Seconds())
	},
}

//nolint:gochecknoinits
func init() {
	rootCmd.AddCommand(abuseCmd)
	abuseCmd.AddCommand(abuseCreateBranchesCmd)
	abuseCreateBranchesCmd.Flags().String("branch-prefix", "abuse-", "prefix to create branches under")
	abuseCreateBranchesCmd.Flags().Bool("clean-only", false, "only clean up past runs")
	abuseCreateBranchesCmd.Flags().Int("amount", 1000000, "amount of things to do")
	abuseCreateBranchesCmd.Flags().Int("parallelism", 100, "amount of things to do in parallel")
}
