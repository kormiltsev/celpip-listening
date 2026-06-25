## Launch audio generator
Keep running:
```
docker run --rm -p 8880:8880 ghcr.io/remsky/kokoro-fastapi-cpu:latest
```
Generate dialogue.json with prompt ```create_Dialogue_as_json_example``` (i'm using ChatGPT Plus (can be used as UI without paying for API calls)). Response should be JSON formatted, save as ```./dialogue.json```.

Generate audio with:
``` 
go run main.go --in ./dialogue.json 
```
That script generates list of wav files in root. Remove them after combined to a dialogue.mp3 file with ffmpeg (will add pauses between lines). Then move all files in new created directory:
```
./{New Created as Title}
   - dialogue.mp3
   - dialogue.json
   - query.txt - created from dialogue.json for human read
   - acswers.txt - created from dialogue.json for human read
```
## use case 1
1. play mp3 (use VLC or any other player)
2. open ```query.txt``` - answer questions
3. verify with correct answers ```answers.txt```

## use case 2
1. launch ```static-server-darwin-arm64``` (if exists). To build from source: 
```
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o static-server-darwin-arm64 static-server.go
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o static-server-linux-amd64 static-server.go  
```
2. open ```http://localhost:8080``` and find directory with test