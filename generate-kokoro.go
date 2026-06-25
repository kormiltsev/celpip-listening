package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const defaultAPIURL = "http://localhost:8880/v1/audio/speech"

type CELPIPTest struct {
	Title     string         `json:"title"`
	Level     string         `json:"level"`
	Part      string         `json:"part"`
	Topic     string         `json:"topic"`
	Audio     AudioConfig    `json:"audio"`
	Dialogue  []DialogueLine `json:"dialogue"`
	Questions []Question     `json:"questions"`
}

type AudioConfig struct {
	SpeechRate          string `json:"speech_rate"`
	SpeechTone          string `json:"speech_tone"`
	InferenceDensity    string `json:"inference_density"`
	AgreementComplexity string `json:"agreement_complexity"`
	ViewpointOverlap    string `json:"viewpoint_overlap"`
	Distractors         string `json:"distractors"`
	DecisionChanges     int    `json:"decision_changes"`
	Interruptions       string `json:"interruptions"`
}

type DialogueLine struct {
	Index   int    `json:"index"`
	Speaker string `json:"speaker"`
	Name    string `json:"name"`
	Voice   string `json:"voice"`
	Text    string `json:"text"`
	PauseMS int    `json:"pause_ms"`
}

type Question struct {
	ID       int               `json:"id"`
	Skill    string            `json:"skill"`
	Question string            `json:"question"`
	Options  map[string]string `json:"options"`
	Answer   Answer            `json:"answer"`
}

type Answer struct {
	CorrectOption string `json:"correct_option"`
	CorrectAnswer string `json:"correct_answer"`
	Explanation   string `json:"explanation"`
}

type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

func main() {
	inPath := flag.String("in", "", "input JSON file; optional, default: all .json files in working directory")
	apiURL := flag.String("api", defaultAPIURL, "Kokoro API URL")
	flag.Parse()

	if *inPath != "" {
		if err := PrepareDialogueJSON(*inPath); err != nil {
			fmt.Fprintf(os.Stderr, "prepare dialogue.json error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := CheckKokoroAvailable(*apiURL); err != nil {
		fmt.Fprintf(os.Stderr, "Kokoro is not available: %v\n", err)
		os.Exit(1)
	}

	files, err := resolveInputFiles(*inPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "input error:", err)
		os.Exit(1)
	}

	for _, file := range files {
		if err := ProcessFile(file, *apiURL); err != nil {
			fmt.Fprintf(os.Stderr, "failed to process %s: %v\n", file, err)
			os.Exit(1)
		}
	}

	fmt.Println("done")
}

func resolveInputFiles(inPath string) ([]string, error) {
	if inPath != "" {
		return []string{inPath}, nil
	}

	files, err := filepath.Glob("*.json")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .json files found in working directory")
	}

	sort.Strings(files)
	return files, nil
}

func ProcessFile(path string, apiURL string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var test CELPIPTest
	if err := json.Unmarshal(data, &test); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if strings.TrimSpace(test.Title) == "" {
		return fmt.Errorf("missing title")
	}

	outDir := sanitizeFileName(test.Title)

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	for i, line := range test.Dialogue {
		if strings.TrimSpace(line.Text) == "" {
			return fmt.Errorf("dialogue line %d has empty text", i+1)
		}
		if strings.TrimSpace(line.Voice) == "" {
			return fmt.Errorf("dialogue line %d has empty voice", i+1)
		}

		req := SpeechRequest{

			Model:          "kokoro",
			Input:          line.Text,
			Voice:          line.Voice,
			ResponseFormat: "wav",
			Speed:          SpeechRateToSpeed(test.Audio.SpeechRate),
		}

		audio, err := GenerateWAV(apiURL, req)
		if err != nil {
			return fmt.Errorf("line %d Kokoro error: %w", i+1, err)
		}

		outFile := filepath.Join(outDir, fmt.Sprintf("%d.wav", i+1))
		if err := os.WriteFile(outFile, audio, 0644); err != nil {
			return err
		}

		fmt.Println("created", outFile)
	}

	if err := MergeDialogueToMP3(outDir, test); err != nil {
		return err
	}

	if err := RemoveWAVFiles(outDir); err != nil {
		return err
	}

	if err := WriteQuestions(filepath.Join(outDir, "query.txt"), test); err != nil {
		return err
	}

	if err := WriteAnswers(filepath.Join(outDir, "answers.txt"), test); err != nil {
		return err
	}

	if err := moveInputFile(path, outDir); err != nil {
		return err
	}

	// different quession pages for Parts 1-3 and Part 4-6
	if err := CopyStaticFiles(outDir, test.Part); err != nil {
		fmt.Println("copying HTML file err", err)
	}

	// audio questions for Parts 1-3
	if test.Part == "Part 1" || test.Part == "Part 2" || test.Part == "Part 3" {
		if err := GenerateQuestionAudio(outDir, test, apiURL); err != nil {
			fmt.Println("audio questions generate error:", err)
		}
	}

	return nil
}

func GenerateWAV(apiURL string, speech SpeechRequest) ([]byte, error) {
	body, err := json.Marshal(speech)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 2 * time.Minute}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer not-needed")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func CheckKokoroAvailable(apiURL string) error {
	u := strings.TrimPrefix(apiURL, "http://")
	u = strings.TrimPrefix(u, "https://")
	host := strings.Split(u, "/")[0]

	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to %s", host)
	}
	_ = conn.Close()

	return nil
}

func WriteQuestions(path string, test CELPIPTest) error {
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n%s | %s | %s\n\n", test.Title, test.Level, test.Part, test.Topic)

	for _, q := range test.Questions {
		fmt.Fprintf(&b, "%d. [%s] %s\n", q.ID, q.Skill, q.Question)

		for _, key := range []string{"A", "B", "C", "D"} {
			fmt.Fprintf(&b, "   %s) %s\n", key, q.Options[key])
		}

		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

func WriteAnswers(path string, test CELPIPTest) error {
	var b strings.Builder

	fmt.Fprintf(&b, "%s - Answers\n\n", test.Title)

	for _, q := range test.Questions {
		fmt.Fprintf(&b, "%d. %s\n", q.ID, q.Question)
		fmt.Fprintf(&b, "Correct option: %s\n", q.Answer.CorrectOption)
		fmt.Fprintf(&b, "Correct answer: %s\n", q.Answer.CorrectAnswer)
		fmt.Fprintf(&b, "Explanation: %s\n\n", q.Answer.Explanation)
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

func moveInputFile(src string, outDir string) error {
	dst := filepath.Join(outDir, filepath.Base(src))

	srcAbs, _ := filepath.Abs(src)
	dstAbs, _ := filepath.Abs(dst)

	if srcAbs == dstAbs {
		return nil
	}

	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("destination JSON already exists: %s", dst)
	}

	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, input, 0644); err != nil {
		return err
	}

	return os.Remove(src)
}

func sanitizeFileName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")

	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	s = re.ReplaceAllString(s, "")

	if s == "" {
		return "celpip_test"
	}

	return s
}

func SpeechRateToSpeed(rate string) float64 {
	switch strings.ToLower(strings.TrimSpace(rate)) {
	case "slow":
		return 1.0
	case "moderate", "normal", "":
		return 1.10
	case "fast":
		return 1.25
	case "very_fast", "very-fast":
		return 1.35
	default:
		return 1.0
	}
}

// merge audio
func MergeDialogueToMP3(outDir string, test CELPIPTest) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found; install it first: brew install ffmpeg")
	}

	tempDir, err := os.MkdirTemp("", "celpip-audio-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	listFile := filepath.Join(tempDir, "concat.txt")
	var list strings.Builder

	for i, line := range test.Dialogue {
		wavPath := filepath.Join(outDir, fmt.Sprintf("%d.wav", i+1))

		if _, err := os.Stat(wavPath); err != nil {
			return fmt.Errorf("missing audio file %s: %w", wavPath, err)
		}

		list.WriteString(fmt.Sprintf("file '%s'\n", escapeFFmpegPath(wavPath)))

		if line.PauseMS > 0 {
			silencePath := filepath.Join(tempDir, fmt.Sprintf("pause_%d.wav", i+1))
			duration := float64(line.PauseMS) / 1000.0

			cmd := exec.Command(
				"ffmpeg",
				"-y",
				"-f", "lavfi",
				"-i", "anullsrc=r=24000:cl=mono",
				"-t", fmt.Sprintf("%.3f", duration),
				"-q:a", "9",
				"-acodec", "pcm_s16le",
				silencePath,
			)

			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to create pause audio: %w\n%s", err, string(output))
			}

			list.WriteString(fmt.Sprintf("file '%s'\n", escapeFFmpegPath(silencePath)))
		}
	}

	if err := os.WriteFile(listFile, []byte(list.String()), 0644); err != nil {
		return err
	}

	outMP3 := filepath.Join(outDir, "dialogue.mp3")

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-codec:a", "libmp3lame",
		"-q:a", "2",
		outMP3,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to merge dialogue audio: %w\n%s", err, string(output))
	}

	fmt.Println("created", outMP3)
	return nil
}

func escapeFFmpegPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	return strings.ReplaceAll(abs, "'", "'\\''")
}

// remove wavs after mp3 ready
func RemoveWAVFiles(outDir string) error {
	files, err := filepath.Glob(filepath.Join(outDir, "*.wav"))
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("failed to remove wav file %s: %w", file, err)
		}

		fmt.Println("removed", file)
	}

	return nil
}

// to generate audio questions
func GenerateQuestionAudio(outDir string, test CELPIPTest, apiURL string) error {
	audioQuestionsDir := filepath.Join(outDir, "audioquestions")

	if err := os.MkdirAll(audioQuestionsDir, 0755); err != nil {
		return fmt.Errorf("create audioquestions dir: %w", err)
	}

	if len(test.Questions) == 0 {
		return fmt.Errorf("no questions found")
	}

	for i, q := range test.Questions {
		text := strings.TrimSpace(q.Question)
		if text == "" {
			return fmt.Errorf("question %d has empty text", i+1)
		}

		req := SpeechRequest{
			Model:          "kokoro",
			Input:          text,
			Voice:          "af_bella",
			ResponseFormat: "mp3",
			Speed:          1.0,
		}

		audio, err := GenerateWAV(apiURL, req)
		if err != nil {
			return fmt.Errorf("question %d Kokoro error: %w", i+1, err)
		}

		outFile := filepath.Join(audioQuestionsDir, fmt.Sprintf("q%02d.mp3", i+1))

		if err := os.WriteFile(outFile, audio, 0644); err != nil {
			return fmt.Errorf("write question audio %s: %w", outFile, err)
		}

		fmt.Println("created", outFile)
	}

	return nil
}

// copy HTML
func CopyStaticFiles(outDir, part string) error {
	files := make([]string, 0)
	if part == "Part 1" || part == "Part 2" || part == "Part 3" {
		files = []string{
			"celpip-listening-1-2-3-index.html",
		}
	} else if part == "Part 4" || part == "Part 5" || part == "Part 6" {
		files = []string{
			"celpip-listening-4-5-6-index.html",
		}
	} else {
		files = []string{
			"celpip-listening-default-index.html",
		}
	}

	for _, name := range files {
		src := name
		dst := filepath.Join(outDir, "index.html")

		input, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", src, err)
		}

		if err := os.WriteFile(dst, input, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", dst, err)
		}

		fmt.Println("copied", dst)
	}

	return nil
}

type QuestionsFile struct {
	Title     string     `json:"title"`
	Level     string     `json:"level"`
	Part      string     `json:"part"`
	Topic     string     `json:"topic"`
	Questions []Question `json:"questions"`
}

func PrepareDialogueJSON(dialoguePath string) error {
	dialoguePath, err := filepath.Abs(dialoguePath)
	if err != nil {
		return err
	}

	dialogueDir := filepath.Dir(dialoguePath)
	questionsPath := filepath.Join(dialogueDir, "questions.json")

	if _, err := os.Stat(dialoguePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("dialogue.json not found: %s", dialoguePath)
		}
		return fmt.Errorf("check dialogue.json: %w", err)
	}

	data, err := os.ReadFile(dialoguePath)
	if err != nil {
		return fmt.Errorf("read dialogue.json: %w", err)
	}

	var test CELPIPTest
	if err := json.Unmarshal(data, &test); err != nil {
		return fmt.Errorf("bad dialogue.json format: %w", err)
	}

	// Already contains questions.
	if len(test.Questions) > 0 {
		return nil
	}

	qData, err := os.ReadFile(questionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("dialogue.json has no questions and %s does not exist", questionsPath)
		}
		return fmt.Errorf("read questions.json: %w", err)
	}

	var qFile QuestionsFile
	if err := json.Unmarshal(qData, &qFile); err != nil {
		return fmt.Errorf("bad questions.json format: %w", err)
	}

	if len(qFile.Questions) == 0 {
		return fmt.Errorf("questions.json has empty questions list")
	}

	if err := ValidateQuestions(qFile.Questions); err != nil {
		return fmt.Errorf("bad questions.json: %w", err)
	}

	test.Questions = qFile.Questions

	updated, err := json.MarshalIndent(test, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal updated dialogue.json: %w", err)
	}

	if err := os.WriteFile(dialoguePath, updated, 0644); err != nil {
		return fmt.Errorf("write updated dialogue.json: %w", err)
	}

	if err := os.Remove(questionsPath); err != nil {
		return fmt.Errorf("remove questions.json: %w", err)
	}

	fmt.Printf("merged %s into %s\n", filepath.Base(questionsPath), filepath.Base(dialoguePath))

	return nil
}

func ValidateQuestions(questions []Question) error {
	for i, q := range questions {
		if q.ID == 0 {
			return fmt.Errorf("question %d has empty id", i+1)
		}

		if strings.TrimSpace(q.Question) == "" {
			return fmt.Errorf("question %d has empty question text", q.ID)
		}

		for _, key := range []string{"A", "B", "C", "D"} {
			if strings.TrimSpace(q.Options[key]) == "" {
				return fmt.Errorf("question %d has empty option %s", q.ID, key)
			}
		}

		correct := strings.TrimSpace(q.Answer.CorrectOption)
		if correct != "A" && correct != "B" && correct != "C" && correct != "D" {
			return fmt.Errorf("question %d has invalid correct_option: %q", q.ID, correct)
		}

		if strings.TrimSpace(q.Answer.CorrectAnswer) == "" {
			return fmt.Errorf("question %d has empty correct_answer", q.ID)
		}

		if strings.TrimSpace(q.Answer.Explanation) == "" {
			return fmt.Errorf("question %d has empty explanation", q.ID)
		}
	}

	return nil
}
