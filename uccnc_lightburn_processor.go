package main

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

/*
var parkLaserPower = 64.0
var minLaserPower = 64.0
var maxLaserPower = 255.0
*/

var parkLaserPower = 51.0
var minLaserPower = 51.0
var maxLaserPower = 255.0
var lightburnMaxValue = 255.0

func main() {
	filenames := os.Args[1:]

	if len(filenames) == 0 {
		var err error
		if filenames, err = WalkMatch(".", "*.nc"); err != nil {
			log.Fatal(err)
		}
	}

	for _, filename := range filenames {
		outfile := filename[:len(filename)-len(filepath.Ext(filename))] + "_UCCNC.nc"
		processFile(filename, outfile)
	}
}

func calcMD5(filename string) string {
	h := md5.New()
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func outfileIncludedHash(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		a := strings.Split(line, "hash:")
		if len(a) != 2 {
			return ""
		}
		b := strings.Split(a[1], " ")
		return b[0]
	}
	return ""
}

func WalkMatch(root, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func processFile(filename, outfile string) {
	if strings.HasSuffix(filename, "_UCCNC.nc") {
		return
	}

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var buf strings.Builder
	var totalLineNum, afterCommentLineNum, commentSectionNum int
	commentSectionNum = 1
	lastLine := ""

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		totalLineNum += 1

		if totalLineNum == 1 {
			if !strings.HasPrefix(line, "; LightBurn ") {
				fmt.Println("skip", filename, "not a valid LightBurn program")
				return
			}

			outhash := outfileIncludedHash(outfile)
			hash := calcMD5(filename)
			if hash == outhash {
				fmt.Println("skip", filename, "already processed")
				return
			}

			fmt.Println("processing", filename)
			fmt.Fprintf(&buf, "( lightburn_processor.go - hash:%s - %s )\n", hash, time.Now().UTC())
		}

		if totalLineNum == 2 {
			if !strings.Contains(line, "LinuxCNC device profile") {
				fmt.Println("skip", filename, "not a valid LightBurn LinuxCNC profile program")
				return
			}
		}

		// comment
		if strings.HasPrefix(line, ";") {
			if afterCommentLineNum > 0 {
				commentSectionNum += 1
			}
			idx := 1 // remove ";"
			if line[0:2] == "; " {
				idx = 2 // remove "; "
			}
			fmt.Fprintf(&buf, "( %s )\n", line[idx:])

			if commentSectionNum == 2 {
				fmt.Fprintln(&buf, "M3") // start the selected spindle clockwise (not needed?)
			}
			afterCommentLineNum = 0
			continue
		}
		afterCommentLineNum += 1

		// laser power
		if strings.HasPrefix(line, "M67 E0 Q") {
			if line == "M67 E0 Q0" && lastLine == "M9" { // disable if M9 was called before this line
				fmt.Fprintln(&buf, "M11") // disable laser
				continue
			}

			a := strings.SplitN(line, " Q", 2)
			if len(a) != 2 {
				log.Fatalf("failed to parse laser power: %v", line)
			}
			v, err := strconv.ParseFloat(a[1], 64)
			if err != nil {
				log.Fatalf("failed to parse laser power: %v %v", line, err)
			}

			scaled := minLaserPower + ((maxLaserPower - minLaserPower) * (v / lightburnMaxValue))
			if v == 0 {
				scaled = parkLaserPower
			}
			//fmt.Fprintf(&buf, "M10 Q%.01f\n", scaled)
			fmt.Fprintf(&buf, "M10 Q%d\n", int(scaled))
			continue
		}

		// gcode start
		if line == "G21" && afterCommentLineNum == 1 {
			fmt.Fprintln(&buf, "G00 G17 G40 G21 G54")
			continue
		}

		fmt.Fprintln(&buf, line)
		lastLine = line
	}

	// gcode end
	fmt.Fprintln(&buf, "M11") // disable laser (again to be sure)
	fmt.Fprintln(&buf, "M5")  // stop the selected spindle (not needed?)
	fmt.Fprintln(&buf, "M2")  // program end

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	if false {
		fmt.Println(buf.String())
	} else {
		if err := os.WriteFile(outfile, []byte(buf.String()), 0644); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("created", outfile)
}
