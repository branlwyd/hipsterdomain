package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
)

type PostfixTree struct {
	subTrees  map[byte]*PostfixTree
	contained bool
}

func (pt *PostfixTree) AddPostfix(postfix string) {
	tree := pt

	for len(postfix) > 0 {
		lastByte := postfix[len(postfix)-1]

		if tree.subTrees == nil {
			tree.subTrees = make(map[byte]*PostfixTree)
		}
		subTree, ok := tree.subTrees[lastByte]
		if !ok {
			subTree = new(PostfixTree)
			tree.subTrees[lastByte] = subTree
		}

		tree = subTree
		postfix = postfix[0 : len(postfix)-1]
	}

	tree.contained = true
}

func (pt *PostfixTree) GetPostfixes(word string) []string {
	var postfixes []string
	tree := pt
	ptr := len(word)

	for {
		if tree.contained {
			postfixes = append(postfixes, word[ptr:len(word)])
		}
		if ptr == 0 {
			return postfixes
		}
		ptr--
		subTree, ok := tree.subTrees[word[ptr]]
		if !ok {
			return postfixes
		}
		tree = subTree
	}
}

func domainExists(name string) (bool, error) {
	nss, err := net.LookupNS(name)
	if err != nil {
		if err, ok := err.(*net.DNSError); ok {
			if err.Err == "no such host" {
				return false, nil
			}
		}
		return false, err
	}
	return len(nss) > 0, nil
}

func getTlds() ([]string, error) {
	resp, err := http.Get("https://data.iana.org/TLD/tlds-alpha-by-domain.txt")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := strings.ToLower(string(bodyBytes))

	return splitToLinesWithComments(body), nil
}

func getWords() ([]string, error) {
	bodyBytes, err := ioutil.ReadFile("/usr/share/dict/words")
	if err != nil {
		return nil, err
	}
	body := strings.ToLower(string(bodyBytes))

	return splitToLinesWithComments(body), nil
}

func splitToLinesWithComments(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func postfixSplit(name string, pt *PostfixTree) []string {
	var results []string

	for _, postfix := range pt.GetPostfixes(name) {
		results = append(results, fmt.Sprintf("%s.%s", name[0:len(name)-len(postfix)], postfix))
	}

	return results
}

func domainHandler(domainChan chan string) {
	for domain := range domainChan {
		exists, err := domainExists(domain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", domain, err)
		} else if !exists {
			fmt.Printf("%s\n", domain)
		}
	}
}

func main() {
	// Get list of words.
	words, err := getWords()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get word list: %s", err)
		return
	}

	// Get list of top-level domains.
	tlds, err := getTlds()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get TLD list: %s", err)
		return
	}

	var tldsPt PostfixTree
	for _, tld := range tlds {
		tldsPt.AddPostfix(tld)
	}

	domainChan := make(chan string)
	for i := 0; i < 100; i++ {
		go domainHandler(domainChan)
	}

	for _, word := range words {
		for _, domain := range postfixSplit(strings.ToLower(word), &tldsPt) {
			domainChan <- domain
		}
	}
	close(domainChan)
}
