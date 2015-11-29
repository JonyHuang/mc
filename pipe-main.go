package main

/*
 * Minio Client, (C) 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.

package main

import (
	"os"
	"syscall"

	"github.com/minio/cli"
	"github.com/minio/minio-xl/pkg/probe"
)

var (
	pipeFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "help, h",
			Usage: "Help of pipe.",
		},
	}
)

// Display contents of a file.
var pipeCmd = cli.Command{
	Name:   "pipe",
	Usage:  "Write contents of stdin to one or more targets. When no target is specified, it writes to stdout.",
	Action: mainPipe,
	Flags:  append(pipeFlags, globalFlags...),
	CustomHelpTemplate: `NAME:
   mc {{.Name}} - {{.Usage}}

USAGE:
   mc {{.Name}} [FLAGS] [TARGET...]

FLAGS:
  {{range .Flags}}{{.}}
  {{end}}
EXAMPLES:
   1. Write contents of stdin to a file on local filesystem.
      $ mc {{.Name}} /tmp/hello-world.go

   2. Write contents of stdin to an object on Amazon S3 cloud storage.
      $ mc {{.Name}} s3.amazonaws.com/personalbuck/meeting-notes.txt

   3. Copy an ISO image to an object on Amazon S3 cloud storage and Google Cloud Storage simultaneously.
      $ cat debian-8.2.iso | mc {{.Name}} s3.amazonaws.com/ferenginar/gnuos.iso storage.googleapis.com/miniocloud/gnuos.iso

   4. Stream MySQL database dump to Amazon S3 directly.
      $ mysqldump -u root -p ******* accountsdb | mc {{.Name}} s3.amazonaws.com/ferenginar/backups/accountsdb-oct-9-2015.sql
`,
}

// pipe writes contents of stdin a collection of URLs.
func pipe(targetURLs []string) *probe.Error {
	if len(targetURLs) == 0 || targetURLs == nil {
		// When no target is specified, pipe cat's stdin to stdout.
		return catOut(os.Stdin).Trace()
	}

	// Stream from stdin to multiple objects until EOF.
	// Ignore size, since os.Stat() would not return proper size all the time
	// for local filesystem for example /proc files.
	err := putTargets(targetURLs, -1, os.Stdin)
	// TODO: See if this check is necessary.
	switch e := err.ToGoError().(type) {
	case *os.PathError:
		if e.Err == syscall.EPIPE {
			// stdin closed by the user. Gracefully exit.
			return nil
		}
	}
	return err.Trace(targetURLs...)
}

// mainPipe is the main entry point for pipe command.
func mainPipe(ctx *cli.Context) {
	// Set global flags from context.
	setGlobalsFromContext(ctx)

	// extract URLs.
	URLs, err := args2URLs(ctx.Args())
	fatalIf(err.Trace(ctx.Args()...), "Unable to parse arguments.")

	err = pipe(URLs)
	fatalIf(err.Trace(URLs...), "Unable to write to one or more targets.")
}
*/
