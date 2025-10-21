package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafov/m3u8"
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

// --- заголовки, близкие к реальным браузерным ---
var (
	defaultUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	defaultRef    = "https://rutube.ru/"
	defaultAccept = "application/json, text/plain, */*"
	defaultALang  = "ru-RU,ru;q=0.9,en;q=0.8"
	defaultOrigin = "https://rutube.ru"
)

// playOptions — минимум, который нам нужен
type playOptions struct {
	Title         string `json:"title"`
	VideoBalancer struct {
		M3u8 string `json:"m3u8"`
	} `json:"video_balancer"`
}

// ExtractMP4 качает ролик по ссылке и возвращает имя файла (без папки)
func ExtractMP4(videoURL string) (string, error) {
	id, err := extractID(videoURL)
	if err != nil {
		return "", err
	}
	opts, err := fetchOptions(id)
	if err != nil {
		return "", err
	}
	if opts.VideoBalancer.M3u8 == "" {
		return "", errors.New("пустой m3u8 в playOptions")
	}

	variantURL, err := pickBestVariant(opts.VideoBalancer.M3u8)
	if err != nil {
		return "", err
	}

	// итоговый путь
	fileName := sanitize(opts.Title) + ".mp4"
	outPath := filepath.Join("downloads", fileName)
	if err := os.MkdirAll("downloads", 0o755); err != nil {
		return "", err
	}

	// ffmpeg сам расшифрует (AES-128), склеит и справится с обрывами
	if err := ffmpegMuxFromM3U8(variantURL, outPath); err != nil {
		return "", err
	}

	// Опциональный автоснос (минуты) из окружения DOWNLOAD_TTL_MIN
	if ttlMin := ttlFromEnv(); ttlMin > 0 {
		go func(p string, minutes int) {
			time.Sleep(time.Duration(minutes) * time.Minute)
			_ = os.Remove(p)
		}(outPath, ttlMin)
	}

	return fileName, nil
}

// --- helpers --------------------------------------------------------------

func extractID(input string) (string, error) {
	input = strings.TrimSpace(input)
	re := regexp.MustCompile(`(?i)^https?://rutube\.ru/video/([a-f0-9]{32})/?$`)
	m := re.FindStringSubmatch(input)
	if len(m) < 2 {
		return "", errors.New("не смог распознать ID видео")
	}
	return m[1], nil
}

func ttlFromEnv() int {
	s := strings.TrimSpace(os.Getenv("DOWNLOAD_TTL_MIN"))
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// общий GET с нужными заголовками
func httpGetWithHeaders(u string) (*http.Response, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Referer", defaultRef)
	req.Header.Set("Accept", defaultAccept)
	req.Header.Set("Accept-Language", defaultALang)
	req.Header.Set("Origin", defaultOrigin)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	return httpClient.Do(req)
}

// 1) init → 2) play/options → 3) HTML fallback
func fetchOptions(id string) (*playOptions, error) {
	// 1) init
	if po, err := fetchOptionsInit(id); err == nil && po.VideoBalancer.M3u8 != "" {
		log.Println("✅ Использован init-эндпоинт")
		return po, nil
	} else if err != nil {
		log.Printf("⚠️ init-эндпоинт не сработал: %v", err)
	}

	// 2) play/options
	if po, err := fetchOptionsPlayOptions(id); err == nil && po.VideoBalancer.M3u8 != "" {
		log.Println("✅ Использован play/options")
		return po, nil
	} else if err != nil {
		log.Printf("⚠️ play/options не сработал: %v", err)
	}

	// 3) HTML fallback
	if po, err := fetchOptionsFromHTML(id); err == nil && po.VideoBalancer.M3u8 != "" {
		log.Println("✅ Использован HTML-фолбэк (video_balancer.m3u8)")
		return po, nil
	} else if err != nil {
		log.Printf("❌ HTML-фолбэк не сработал: %v", err)
	}

	return nil, errors.New("не удалось получить m3u8 ни из init, ни из play/options, ни из HTML")
}

func fetchOptionsInit(id string) (*playOptions, error) {
	u := fmt.Sprintf("https://rutube.ru/api/video/%s/init", id)
	resp, err := httpGetWithHeaders(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("init http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var full map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&full); err != nil {
		return nil, err
	}

	vb, _ := full["video_balancer"].(map[string]any)
	m3u8url, _ := vb["m3u8"].(string)
	if m3u8url == "" {
		return nil, errors.New("m3u8 не найден в init")
	}
	title, _ := full["title"].(string)

	return &playOptions{
		Title: title,
		VideoBalancer: struct {
			M3u8 string `json:"m3u8"`
		}{M3u8: m3u8url},
	}, nil
}

func fetchOptionsPlayOptions(id string) (*playOptions, error) {
	u := fmt.Sprintf("https://rutube.ru/api/play/options/%s/?no_404=true&referer=https%%3A%%2F%%2Frutube.ru", id)
	resp, err := httpGetWithHeaders(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("play/options http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var po playOptions
	if err := json.NewDecoder(resp.Body).Decode(&po); err != nil {
		return nil, err
	}
	if po.VideoBalancer.M3u8 == "" {
		return nil, errors.New("video_balancer.m3u8 пустой (play/options)")
	}
	return &po, nil
}

// HTML fallback — вытаскиваем video_balancer.m3u8 из инлайнового JSON на странице
func fetchOptionsFromHTML(id string) (*playOptions, error) {
	pageURL := "https://rutube.ru/video/" + id + "/"
	resp, err := httpGetWithHeaders(pageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("html http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// ищем блок "video_balancer":{...}
	reVB := regexp.MustCompile(`"video_balancer"\s*:\s*\{[^}]+\}`)
	vb := reVB.Find(body)
	if vb == nil {
		return nil, errors.New("video_balancer не найден в HTML")
	}

	// m3u8 внутри video_balancer
	reM3U8 := regexp.MustCompile(`"m3u8"\s*:\s*"([^"]+)"`)
	m := reM3U8.FindSubmatch(vb)
	if len(m) < 2 {
		return nil, errors.New("m3u8 не найден в video_balancer")
	}
	m3u8url := string(m[1])
	// HTML экранирует & как \u0026 — вернём
	m3u8url = strings.ReplaceAll(m3u8url, `\u0026`, `&`)

	// Заголовок видео (не критично, если не найдём)
	title := ""
	reTitle := regexp.MustCompile(`"title"\s*:\s*"([^"]+)"`)
	if t := reTitle.FindSubmatch(body); len(t) >= 2 {
		title = string(t[1])
	}

	return &playOptions{
		Title: title,
		VideoBalancer: struct {
			M3u8 string `json:"m3u8"`
		}{M3u8: m3u8url},
	}, nil
}

// pickBestVariant — если master, берём самый "жирный" вариант; если media — возвращаем как есть
func pickBestVariant(m3u8url string) (string, error) {
	resp, err := httpGetWithHeaders(m3u8url)
	if err != nil {
		return "", fmt.Errorf("ошибка загрузки m3u8: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("m3u8 http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	// читаем в буфер, чтобы можно было пробовать и master, и media
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// сначала пробуем как master
	if mpl, err := tryDecodeMaster(data); err == nil && len(mpl.Variants) > 0 {
		sort.Slice(mpl.Variants, func(i, j int) bool {
			return mpl.Variants[i].Bandwidth > mpl.Variants[j].Bandwidth
		})
		best := mpl.Variants[0].URI
		return resolveURL(m3u8url, best), nil
	}

	// возможно, это media — ок, вернём исходный URL
	if _, err := tryDecodeMedia(data); err == nil {
		log.Println("⚠️ M3U8 не содержит вариантов, используем напрямую как media")
		return m3u8url, nil
	}

	// ни master, ни media — странно, вернём кусок плейлиста для отладки
	sample := string(data)
	if len(sample) > 200 {
		sample = sample[:200]
	}
	return "", fmt.Errorf("не удалось распарсить плейлист (ни master, ни media). фрагмент: %q", sample)
}

func tryDecodeMaster(b []byte) (*m3u8.MasterPlaylist, error) {
	mpl := m3u8.NewMasterPlaylist()
	err := mpl.DecodeFrom(bytes.NewReader(b), true)
	return mpl, err
}

func tryDecodeMedia(b []byte) (*m3u8.MediaPlaylist, error) {
	pl, typ, err := m3u8.DecodeFrom(bytes.NewReader(b), true)
	if err != nil {
		return nil, err
	}
	if typ != m3u8.MEDIA {
		return nil, errors.New("это не media playlist")
	}
	return pl.(*m3u8.MediaPlaylist), nil
}

func ffmpegMuxFromM3U8(m3u8url, outPath string) error {
	ffmpegPath := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegPath = "ffmpeg/bin/ffmpeg.exe"
	}
	if _, err := exec.LookPath(ffmpegPath); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("ffmpeg не найден в PATH: %w", err)
	}

	args := []string{
		"-y",

		// сети и HLS
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-allowed_extensions", "ALL",

		// устойчивость к обрывам
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_on_network_error", "1",
		"-rw_timeout", "30000000", // 30s в микросекундах

		// заголовки
		"-user_agent", defaultUA,
		"-referer", defaultRef,

		// вход
		"-i", m3u8url,

		// без перекодирования
		"-c", "copy",

		outPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	// Хотите отладку в логи сервера — можно склеить вывод в буфер и вернуть в ошибке.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resolveURL(master, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	mu, _ := url.Parse(master)
	ru, _ := url.Parse(ref)
	return mu.ResolveReference(ru).String()
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	re := regexp.MustCompile(`[<>:"/\\|?*]+`)
	s = re.ReplaceAllString(s, "_")
	s = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
	const maxLength = 80
	runes := []rune(s)
	if len(runes) > maxLength {
		s = string(runes[:maxLength])
	}
	if s == "" {
		s = fmt.Sprintf("rutube_%d", time.Now().Unix())
	}
	return s
}

// ExtractMP4WithProgress — то же, что ExtractMP4, но коллбеком репортит прогресс (секунды из ffmpeg / общая длительность).
func ExtractMP4WithProgress(videoURL string, onProgress func(doneSec, totalSec float64)) (string, error) {
	id, err := extractID(videoURL)
	if err != nil {
		return "", err
	}
	opts, err := fetchOptions(id)
	if err != nil {
		return "", err
	}
	variantURL, err := pickBestVariant(opts.VideoBalancer.M3u8)
	if err != nil {
		return "", err
	}

	// Считаем длительность по media-плейлисту
	totalDur, err := totalDurationSeconds(variantURL)
	if err != nil {
		// не критично — просто не сможем показать проценты
		totalDur = 0
	}

	fileName := sanitize(opts.Title) + ".mp4"
	outPath := filepath.Join("downloads", fileName)
	if err := os.MkdirAll("downloads", 0o755); err != nil {
		return "", err
	}

	if err := ffmpegMuxFromM3U8WithProgress(variantURL, outPath, totalDur, onProgress); err != nil {
		return "", err
	}
	return fileName, nil
}

// totalDurationSeconds скачивает media m3u8 и суммирует EXTINF
func totalDurationSeconds(m3u8url string) (float64, error) {
	resp, err := httpGetWithHeaders(m3u8url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("m3u8 http %d", resp.StatusCode)
	}
	parsed, typ, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return 0, err
	}
	if typ != m3u8.MEDIA {
		// если вдруг мастер — возьмём лучший и повторим
		if mp, ok := parsed.(*m3u8.MasterPlaylist); ok && len(mp.Variants) > 0 {
			best := mp.Variants[0].URI
			return totalDurationSeconds(resolveURL(m3u8url, best))
		}
		return 0, fmt.Errorf("ожидался media playlist")
	}
	mp := parsed.(*m3u8.MediaPlaylist)
	var sum float64
	for _, s := range mp.Segments {
		if s != nil {
			sum += s.Duration
		}
	}
	return sum, nil
}

func ffmpegMuxFromM3U8WithProgress(m3u8url, outPath string, totalDur float64, onProgress func(done, total float64)) error {
	ffmpegPath := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegPath = "ffmpeg/bin/ffmpeg.exe"
	}
	if _, err := exec.LookPath(ffmpegPath); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("ffmpeg не найден в PATH: %w", err)
	}

	args := []string{
		"-y",
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-user_agent", defaultUA,
		"-referer", defaultRef,
		// прогресс в stdout раз в 1с
		"-stats_period", "1",
		"-progress", "pipe:1",
		"-i", m3u8url,
		"-c", "copy",
		outPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	// читаем строки вида: out_time_ms=1234567, progress=...
	go func() {
		buf := make([]byte, 32*1024)
		var chunk []byte
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk = append(chunk, buf[:n]...)
				// парсим по строкам
				for {
					i := strings.IndexByte(string(chunk), '\n')
					if i < 0 {
						break
					}
					line := strings.TrimSpace(string(chunk[:i]))
					chunk = chunk[i+1:]
					if strings.HasPrefix(line, "out_time_ms=") {
						msStr := strings.TrimPrefix(line, "out_time_ms=")
						if ms, e := parseFloat(msStr); e == nil {
							sec := ms / 1000000.0
							if onProgress != nil {
								onProgress(sec, totalDur)
							}
						}
					}
				}
			}
			if err != nil {
				break
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		return err
	}
	// финальный вызов на 100% (если totalDur известен)
	if onProgress != nil && totalDur > 0 {
		onProgress(totalDur, totalDur)
	}
	return nil
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}
