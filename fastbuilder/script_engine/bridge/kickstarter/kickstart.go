//go:build with_v8
// +build with_v8

package script_kickstarter

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"phoenixbuilder/fastbuilder/script_engine"
	"phoenixbuilder/fastbuilder/script_engine/bridge"
	v8 "rogchap.com/v8go"
	"strings"
	"time"
)

func LoadScript(scriptPath string, hb bridge.HostBridge) (func(), error) {
	iso := v8.NewIsolate()
	global := v8.NewObjectTemplate(iso)
	scriptPath = strings.TrimSpace(scriptPath)
	if scriptPath == "" {
		return nil, fmt.Errorf("Empty script path!")
	}
	fmt.Printf("Loading script: %s\n", scriptPath)
	fmt.Printf("JS engine vesion: %v\n", script_engine.JSVERSION)
	var script string
	var scriptName string

	file, fileErr := os.OpenFile(scriptPath, os.O_RDONLY, 0755)
	if fileErr == nil {
		_, scriptName = path.Split(scriptPath)
		scriptData, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}
		script = string(scriptData)
	} else {
		urlPath, err := url.ParseRequestURI(scriptPath)
		if err == nil {
			scriptName = urlPath.Path
			fmt.Printf("It seems to be a url, try loading it...\n")
			result, err := obtainPageContent(scriptPath, 30*time.Second)
			if err != nil {
				return nil, err
			}
			script = string(result)
		} else {
			return nil, fmt.Errorf("script %v \nis neither a valid file %v,\nnor a valid url %v", scriptPath, fileErr, err)
		}
	}

	hasher := sha256.New()
	hasher.Write([]byte(script))
	identifyStr := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	stopFunc := script_engine.InitHostFns(iso, global, hb, scriptName, identifyStr, scriptPath)
	ctx := v8.NewContext(iso, global)
	script_engine.CtxFunctionInject(ctx)
	go func() {
		_, err := ctx.RunScript(script, scriptPath)
		if err != nil {
			e := err.(*v8.JSError) // JavaScript errors will be returned as the JSError struct
			fmt.Printf("Script %s ran into a runtime error, stack dump:\n", scriptPath)
			fmt.Println(e.Message)    // the message of the exception thrown
			fmt.Println(e.Location)   // the filename, line number and the column where the error occured
			fmt.Println(e.StackTrace) // the full stack trace of the error, if available
		}
		//fmt.Printf("Script %s Successfully Loaded, Additional info(%v)\n",scriptPath,finalVal)
	}()
	return stopFunc, nil
}

func obtainPageContent(pageUrl string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(pageUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var buffer [512]byte
	result := bytes.NewBuffer(nil)
	for {
		n, err := resp.Body.Read(buffer[0:])
		result.Write(buffer[0:n])
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	return result.Bytes(), nil
}
