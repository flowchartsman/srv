package main

import (
	"bufio"
	"context"
	"net/http"
	"strings"

	"andy.dev/srv"
	"github.com/peterbourgon/ff/v4"
)

type jobConfig struct {
	wcc  *WebCountClient
	urls []string
}

func main() {
	srv.Declare(srv.ServiceInfo{
		Name:   "flags_example",
		System: "srv examples",
	})

	cfg := &jobConfig{}
	srv.FlagStringListVar(&cfg.urls, "url", "a url to scan")
	srv.FlagsValue(&cfg.wcc, WebCountClientProvider)
	srv.ParseFlags()

	srv.AddJobFn(getURLs, cfg)
	srv.Serve()
}

func getURLs(ctx context.Context, log *srv.Logger, cfg *jobConfig) error {
	log.Infof("I am going to count the occurrences of %s in the following URLS: %q", cfg.wcc.GetWord(), cfg.urls)
	for _, url := range cfg.urls {
		count, err := cfg.wcc.Get(ctx, url)
		if err != nil {
			return err
		}
		log.Info("got URL count", "url", url, "count", count)
	}
	return nil
}

func WebCountClientProvider() (*ff.CoreFlags, func() (*WebCountClient, error)) {
	flags := ff.NewFlags("WebCountClient")
	countWord := flags.StringLong("countword", "", "a word to count")
	return flags, func() (*WebCountClient, error) {
		return NewWebCountClient(*countWord)
	}
}

type WebCountClient struct {
	countWord string
}

func NewWebCountClient(countWord string) (*WebCountClient, error) {
	return &WebCountClient{
		countWord: countWord,
	}, nil
}

func (w *WebCountClient) Get(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return -1, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1, err
	}
	count := 0
	s := bufio.NewScanner(resp.Body)
	s.Split(bufio.ScanWords)
	for s.Scan() {
		word := s.Text()
		if strings.ToLower(word) == w.countWord {
			count++
		}
	}
	return count, nil
}

func (w *WebCountClient) GetWord() string {
	return w.countWord
}
