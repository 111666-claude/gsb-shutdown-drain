// Command shutdown-drain 跑优雅关服样例。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"example.com/drain"
)

// Run 执行一次命令行调用，返回退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("shutdown-drain", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sample := flags.String("sample", "timeout", "timeout / idem / dup / scan")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *sample == "timeout" {
		server := drain.NewServer([]string{"a", "b"})
		server.Begin(0)
		// 才过一秒，远没到 30 秒宽限期。
		fmt.Fprintf(stdout, "kicked=%d active=%d\n", server.Drain(1000), server.Active())
		return 0
	}
	if *sample == "idem" {
		server := drain.NewServer([]string{"a"})
		server.Begin(0)
		fmt.Fprintf(stdout, "second=%v\n", server.Begin(100))
		return 0
	}
	if *sample == "dup" {
		server := drain.NewServer([]string{"a", "b"})
		server.Close([]string{"a"}, 0)
		fmt.Fprintf(stdout, "kicked=%d\n", server.Close([]string{"a"}, 10))
		return 0
	}
	if *sample == "scan" {
		conns := make([]string, 0, 3000)
		for index := 0; index < 3000; index++ {
			conns = append(conns, fmt.Sprintf("c-%d", index))
		}
		server := drain.NewServer(conns)
		for query := 0; query < 3000; query++ {
			server.Active()
		}
		fmt.Fprintf(stdout, "scanned=%d\n", server.Scanned())
		return 0
	}
	fmt.Fprintln(stderr, "需要 --sample timeout|idem|dup|scan")
	return 2
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
