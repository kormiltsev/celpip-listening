# Local run with kokoro and manual ChatGPT prompting
requirements:
```
docker
any WebUI AI chat (tested on ChatGPT Plus)
Linux or Mac
```
Launch and keep runing during generating:
```
docker run --rm -p 8880:8880 ghcr.io/remsky/kokoro-fastapi-cpu:latest
```

1. Request ChatGPT to generate topic. Copy prompt from ```prompts/generate_topic```
2. Send request to the same chat copy-past from ```prompts/generate_CLB_9_Part_3_dialogue``` - select SLB level and Part number. 
```
Part	Official Name	Typical Format	Main Skills
Part 1	Listening to Problem Solving	2-person conversation	Problem solving, details, decisions
Part 2	Listening to a Daily Life Conversation	Casual conversation	Main idea, details, inference
Part 3	Listening for Information	Information exchange	Facts, instructions, details
Part 4	Listening to a News Item	News report	Main idea, sequence, cause/effect
Part 5	Listening to a Discussion	3–4 speakers discussing a topic	Opinions, agreement/disagreement
Part 6	Listening to Viewpoints	Two opposing viewpoints	Compare opinions, inference, attitude
```
3. Save response (JSON) to the file "dialogue.json" in project root directory (where all scripts are) 
4. Send request to the same chat copy-past from ```prompts/generate_CLB_9_Part_3_questions``` - the same prompt but "questions"
5. Save JSON response to the file "questions.json" in project root (next to "dialogue.json")
6. Run
```
generate_kokoro-darwin-arm64
```
(if not built yet)
```
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o generate-kokoro-darwin-arm64 generate-kokoro.go
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o generate-kokoro-linux-amd64 generate-kokoro.go
```
or run with golang:
```
go run generate_kokoro.go --in ./dialogue.json  
```
This will create new directory with:
```
./{New Created as Title}
   - dialogue.mp3 - listening speech
   - dialogue.json - (dialogue.json and questions.json merged)
   - query.txt - human-friendly text file with questions
   - answers.txt - human-friendly text file with answers
   - [dir] audioquestions - directory with audio questions (as for Part 1-3 listening)
   - index.html - to use as a web page http://localhost:8080 (required server to be launched, see below)
```

## use case 1
1. play ```dialogue.mp3``` in player (for ex: VLC) 
2. open ```query.txt``` - answer questions
3. verify with correct answers ```answers.txt```


## use case 2
1. launch ```static-server-darwin-arm64``` (if exists). To build from source: 
```
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o static-server-darwin-arm64 static-server.go
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o static-server-linux-amd64 static-server.go  
```
2. open ```http://localhost:8080``` and find directory with test