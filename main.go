package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/joho/godotenv"
	"github.com/pieterclaerhout/go-log"
)

func main() {
	logger()
	initEnv()
	var cargs = [30]string{}
	args := os.Args[1:]
	for index := range args {
		if index > 29 {
			break
		}
		cargs[index] = args[index]
	}
	if len(args) == 0 {
		help()
		return
	}
	switch cargs[0] {
	case "ps":
		execFunc("docker", []string{"ps"}, true)
		return
	case "kill":
		if cargs[1] == "all" {
			execFunc("docker", []string{"ps", "--format", "{{.Names}}"}, false)
			if len(bufferVal) > 0 {
				execFunc("docker", append([]string{"kill"}, bufferVal...), true)
			}
		} else if cargs[1] == "project" || cargs[1] == "pp" || cargs[1] == "this" || cargs[1] == "self" {
			execFunc("docker", []string{"ps", "--filter", fmt.Sprintf(`"name=%s"`, os.Getenv("PROJECT_PREFIX"))}, false)
			if len(bufferVal) > 0 {
				execFunc("docker", append([]string{"kill"}, bufferVal...), true)
			}
		} else if cargs[1] != "" {
			name, err := getContainerName(cargs[1])
			if err != nil {
				log.Error(err)
				return
			}
			execFunc("docker", []string{"kill", name}, true)
		}
	case "make":
		makeF(cargs[1])
		return
	case "db":
		fmt.Println(fmt.Sprintf("docker exec -it %s_%s %s -u %s -p%s", os.Getenv("PROJECT_PREFIX"), os.Getenv("DB_CONNECTION"), os.Getenv("DB_CONNECTION"), os.Getenv("DB_USERNAME"), os.Getenv("DB_PASSWORD")))
		return
	case "build":
		execFunc("docker-compose", []string{"build"}, true)
		return
	case "up":
		if cargs[1] == "silent" || cargs[1] == "d" || cargs[1] == "-d" {
			execFunc("docker-compose", []string{"-p", os.Getenv("PROJECT_PREFIX"), "up", "-d"}, true)
		} else {
			execFunc("docker-compose", []string{"-p", os.Getenv("PROJECT_PREFIX"), "up"}, true)
		}
		return
	case "down":
		if cargs[1] == "full" {
			execFunc("docker-compose", []string{"-p", os.Getenv("PROJECT_PREFIX"), "down", "--rmi", "local"}, true)
			return
		} else {
			execFunc("docker-compose", []string{"-p", os.Getenv("PROJECT_PREFIX"), "down"}, true)
			return
		}
	case "fulldown":
		execFunc("docker-compose", []string{"-p", os.Getenv("PROJECT_PREFIX"), "down", "--rmi", "local"}, true)
		return
	case "run_php":
		if cargs[1] == "" {
			execFunc("docker", []string{"exec", "-u", "www-data", "-it", os.Getenv("PROJECT_PREFIX") + "_php", "bash"}, true)
			return
		} else {
			execFunc("docker", []string{"exec", "-i", os.Getenv("PROJECT_PREFIX") + "_php", "su", "www-data", "-c", fmt.Sprintf(`"%s"`, "cd /var/www/html/;$command")}, true)
			return
		}
	case "run":
		container, err := getContainerName(cargs[1])
		if err != nil {
			log.Error(fmt.Sprintf("Container %s_%s not exist!", os.Getenv("PROJECT_PREFIX"), cargs[1]))
			return
		}
		execFunc("docker", []string{"exec", "-it", container, "bash"}, true)
		return
	}
}

func initEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Error(err)
	}
}

func getContainerName(name string) (string, error) {
	if containerExist(name) {
		return name, nil
	} else if containerExist(createContainerName(name)) {
		return createContainerName(name), nil
	}

	return "", errors.New("Container not exist")
}

func makeF(arg string) bool {
	if arg == "env" {
		_, err := copy(".env.example", ".env")
		if err != nil {
			log.Error(err)
		} else {
			return true
		}
	}
	return false
}

func help() {
	fmt.Println("HELP:")
	fmt.Println("make env - copy .env.example to .env")
	fmt.Println("build - make docker build")
	fmt.Println("up - docker up in console")
	fmt.Println("up silent/-d/d - docker up daemon")
	fmt.Println("down - docker down")
	fmt.Println("kill all - docker kill all")
	fmt.Println("kill self - docker kill images filtered by project name")
	fmt.Println("kill {{container(php)}} - docker kill image by name")
	fmt.Println("down full - docker down and remove local images")
	fmt.Println("fulldown - docker down and remove local images")
	fmt.Println("run_php - run in php container from project root")
	fmt.Println("run {{container(php)}} - run bash in container")
}

func createContainerName(name string) string {
	return fmt.Sprintf(`%s_%s`, os.Getenv("PROJECT_PREFIX"), name)
}

func containerExist(containerName string) bool {
	execFunc("docker", []string{"ps", "--format", `{{.Names}}`}, false)
	for i := range bufferVal {
		if strings.Trim(bufferVal[i], "\"\n ") == containerName {
			return true
		}
	}

	return false
}

func logger() {
	// Print the log timestamps
	log.PrintTimestamp = false
}

func execFunc(command string, args []string, isLog bool) {
	if !isLog {
		freshBuffer()
	} else {
		log.Info(fmt.Sprintf("%s %s", command, strings.Join(args, " ")))
	}
	cmd := exec.Command(command, args...)

	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	done := make(chan struct{})

	scanner := bufio.NewScanner(r)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			iLogger(line, isLog)
		}
		done <- struct{}{}
	}()
	err := cmd.Start()
	log.CheckError(err)
	<-done
	err = cmd.Wait()
	log.CheckError(err)
}

func iLogger(text string, isLog bool) {
	if isLog {
		log.Info(text)
		return
	}
	buffer(text)
}

var bufferVal []string

func freshBuffer() {
	bufferVal = []string{}
}

func buffer(text string) {
	bufferVal = append(bufferVal, text)
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}
