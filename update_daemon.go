package main

import (
    "fmt"
    "regexp"
)

func Check() {
    re := regexp.MustCompile(`https://djinni\.co/jobs/(\d+-[a-zA-Z0-9-]+)`)
    match := re.FindStringSubmatch("https://djinni.co/jobs/838183-python-ai-engineer-remote/")
    fmt.Println(match)
}
