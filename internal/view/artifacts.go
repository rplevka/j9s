// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// ArtifactsView displays build artifacts.
type ArtifactsView struct {
	*tview.Flex
	app       *App
	table     *ui.Table
	actions   *ui.KeyActions
	jobName   string
	buildNum  int
	artifacts []client.Artifact
}

// NewArtifactsView returns a new artifacts view.
func NewArtifactsView(app *App, jobName string, buildNum int) *ArtifactsView {
	v := &ArtifactsView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  jobName,
		buildNum: buildNum,
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.refresh()
	return v
}

// Name returns the view name.
func (v *ArtifactsView) Name() string {
	return fmt.Sprintf("Artifacts[%s#%d]", v.jobName, v.buildNum)
}

// Hints returns the view hints.
func (v *ArtifactsView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *ArtifactsView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)

	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("View", v.viewCmd, true),
		ui.KeyV:        ui.NewKeyAction("View", v.viewCmd, true),
		ui.KeyD:        ui.NewKeyAction("Download", v.downloadCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
		ui.KeyY:        ui.NewKeyAction("Copy URL", v.copyURLCmd, true),
	})

	v.table.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		key := evt.Key()
		if key == tcell.KeyRune {
			key = tcell.Key(evt.Rune())
		}
		if action, ok := v.actions.Get(key); ok {
			return action.Action(evt)
		}
		return evt
	})
}

func (v *ArtifactsView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		artifacts, err := v.app.Client().GetBuildArtifacts(context.Background(), v.jobName, v.buildNum)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.artifacts = artifacts
			v.renderArtifacts(artifacts)
		})
	}()
}

func (v *ArtifactsView) renderArtifacts(artifacts []client.Artifact) {
	v.table.SetHeaders([]string{"NAME", "PATH", "SIZE"})

	rows := make([][]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		// Determine file type icon
		icon := getFileIcon(artifact.FileName)
		
		rows = append(rows, []string{
			icon + " " + artifact.FileName,
			artifact.RelativePath,
			"-", // Size not available from Jenkins API directly
		})
	}

	v.table.SetData(rows)
	v.table.SetTitle(fmt.Sprintf("Artifacts:%s#%d", v.jobName, v.buildNum))
	v.table.Refresh()
}

// getFileIcon returns an icon based on file extension.
func getFileIcon(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt", ".log", ".md", ".rst":
		return "📄"
	case ".xml", ".json", ".yaml", ".yml", ".toml":
		return "📋"
	case ".html", ".htm":
		return "🌐"
	case ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z":
		return "📦"
	case ".jar", ".war", ".ear":
		return "☕"
	case ".rpm", ".deb":
		return "📀"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico":
		return "🖼️"
	case ".pdf":
		return "📕"
	case ".sh", ".bash", ".py", ".rb", ".pl":
		return "📜"
	case ".exe", ".dll", ".so", ".dylib":
		return "⚙️"
	default:
		return "📁"
	}
}

// isTextFile checks if a file is likely a text file based on extension.
func isTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	textExtensions := map[string]bool{
		".txt": true, ".log": true, ".md": true, ".rst": true,
		".xml": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true,
		".html": true, ".htm": true, ".css": true, ".js": true,
		".sh": true, ".bash": true, ".py": true, ".rb": true, ".pl": true,
		".java": true, ".go": true, ".c": true, ".cpp": true, ".h": true,
		".properties": true, ".ini": true, ".cfg": true, ".conf": true,
		".csv": true, ".tsv": true,
		"": true, // Files without extension might be text
	}
	return textExtensions[ext]
}

func (v *ArtifactsView) getSelectedArtifact() *client.Artifact {
	row, _ := v.table.GetSelection()
	if row <= 0 || row > len(v.artifacts) {
		return nil
	}
	return &v.artifacts[row-1] // -1 for header row
}

func (v *ArtifactsView) viewCmd(*tcell.EventKey) *tcell.EventKey {
	artifact := v.getSelectedArtifact()
	if artifact == nil {
		return nil
	}

	// Check if it's a viewable text file
	if !isTextFile(artifact.FileName) {
		v.app.Flash().Warn(fmt.Sprintf("Cannot view binary file: %s. Use 'd' to download.", artifact.FileName))
		return nil
	}

	// Download and display the artifact
	go func() {
		v.app.QueueUpdateDraw(func() {
			v.app.Flash().Info(fmt.Sprintf("Loading %s...", artifact.FileName))
		})

		content, err := v.app.Client().DownloadArtifact(context.Background(), v.jobName, v.buildNum, artifact.RelativePath)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			// Create artifact viewer
			viewer := NewArtifactViewer(v.app, artifact.FileName, string(content))
			v.app.Content.Push(viewer)
		})
	}()

	return nil
}

func (v *ArtifactsView) downloadCmd(*tcell.EventKey) *tcell.EventKey {
	artifact := v.getSelectedArtifact()
	if artifact == nil {
		return nil
	}

	go func() {
		v.app.QueueUpdateDraw(func() {
			v.app.Flash().Info(fmt.Sprintf("Downloading %s...", artifact.FileName))
		})

		content, err := v.app.Client().DownloadArtifact(context.Background(), v.jobName, v.buildNum, artifact.RelativePath)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		// Save to home directory
		homeDir, _ := os.UserHomeDir()
		path := filepath.Join(homeDir, artifact.FileName)

		if err := os.WriteFile(path, content, 0644); err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.app.Flash().Info(fmt.Sprintf("Downloaded to %s (%s)", path, formatBytes(int64(len(content)))))
		})
	}()

	return nil
}

func (v *ArtifactsView) copyURLCmd(*tcell.EventKey) *tcell.EventKey {
	artifact := v.getSelectedArtifact()
	if artifact == nil {
		return nil
	}

	url := v.app.Client().GetArtifactURL(v.jobName, v.buildNum, artifact.RelativePath)
	if err := clipboardWrite(url); err != nil {
		v.app.Flash().Err(err)
		return nil
	}
	v.app.Flash().Info("URL copied to clipboard")
	return nil
}

func (v *ArtifactsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

// ArtifactViewer displays artifact content.
type ArtifactViewer struct {
	*tview.Flex
	app        *App
	textView   *tview.TextView
	actions    *ui.KeyActions
	filename   string
	content    string
	wrapEnabled bool
	fullScreen  bool
}

// NewArtifactViewer returns a new artifact viewer.
func NewArtifactViewer(app *App, filename, content string) *ArtifactViewer {
	v := &ArtifactViewer{
		Flex:        tview.NewFlex().SetDirection(tview.FlexRow),
		app:         app,
		textView:    tview.NewTextView(),
		actions:     ui.NewKeyActions(),
		filename:    filename,
		content:     content,
		wrapEnabled: true,
	}

	v.textView.SetDynamicColors(true)
	v.textView.SetScrollable(true)
	v.textView.SetBackgroundColor(tcell.ColorDefault)
	v.textView.SetWrap(v.wrapEnabled)
	v.textView.SetBorderPadding(0, 0, 1, 1)
	v.textView.SetText(content)

	// Add border and title
	v.SetBorder(true)
	v.SetBorderColor(tcell.ColorAqua)
	v.SetTitleColor(tcell.ColorAqua)
	v.SetTitleAlign(tview.AlignLeft)

	v.AddItem(v.textView, 0, 1, true)
	v.bindKeys()
	v.updateTitle()

	return v
}

// Name returns the view name.
func (v *ArtifactViewer) Name() string {
	return fmt.Sprintf("Artifact[%s]", v.filename)
}

// Hints returns the view hints.
func (v *ArtifactViewer) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *ArtifactViewer) bindKeys() {
	v.actions.Bulk(ui.KeyMap{
		ui.KeyQuestion: ui.NewKeyAction("Help", func(*tcell.EventKey) *tcell.EventKey {
			v.app.Content.Push(NewHelpView(v.app, v.actions))
			return nil
		}, true),
		ui.KeySlash:   ui.NewKeyAction("Filter", v.filterCmd, true),
		ui.KeyW:       ui.NewKeyAction("Wrap", v.toggleWrapCmd, true),
		ui.KeyF:       ui.NewKeyAction("FullScreen", v.toggleFullScreenCmd, true),
		ui.KeyC:       ui.NewKeyAction("Copy", v.copyCmd, true),
		tcell.KeyCtrlS: ui.NewKeyAction("Save", v.saveCmd, true),
		ui.KeyQ:       ui.NewKeyAction("Back", v.backCmd, true),
		tcell.KeyEsc:  ui.NewKeyAction("Back", v.backCmd, false),
	})

	v.textView.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		key := evt.Key()
		if key == tcell.KeyRune {
			key = tcell.Key(evt.Rune())
		}
		if action, ok := v.actions.Get(key); ok {
			return action.Action(evt)
		}
		return evt
	})
}

func (v *ArtifactViewer) updateTitle() {
	title := fmt.Sprintf(" [aqua::b]%s[white::d] (%s) ", v.filename, formatBytes(int64(len(v.content))))
	
	var indicators []string
	if v.wrapEnabled {
		indicators = append(indicators, "[yellow::b]⏎")
	}
	if v.fullScreen {
		indicators = append(indicators, "[aqua::b]□")
	}
	if len(indicators) > 0 {
		title += "[white::d]" + strings.Join(indicators, " ")
	}
	
	v.SetTitle(title)
}

func (v *ArtifactViewer) filterCmd(*tcell.EventKey) *tcell.EventKey {
	v.app.filterCmd(nil)
	return nil
}

func (v *ArtifactViewer) toggleWrapCmd(*tcell.EventKey) *tcell.EventKey {
	v.wrapEnabled = !v.wrapEnabled
	v.textView.SetWrap(v.wrapEnabled)
	if v.wrapEnabled {
		v.app.Flash().Info("Line wrap enabled")
	} else {
		v.app.Flash().Info("Line wrap disabled")
	}
	v.updateTitle()
	return nil
}

func (v *ArtifactViewer) toggleFullScreenCmd(*tcell.EventKey) *tcell.EventKey {
	v.fullScreen = !v.fullScreen
	v.SetFullScreen(v.fullScreen)
	v.SetBorder(!v.fullScreen)
	if v.fullScreen {
		v.textView.SetBorderPadding(0, 0, 0, 0)
		v.app.Flash().Info("Full screen enabled")
	} else {
		v.textView.SetBorderPadding(0, 0, 1, 1)
		v.app.Flash().Info("Full screen disabled")
	}
	v.updateTitle()
	return nil
}

func (v *ArtifactViewer) copyCmd(*tcell.EventKey) *tcell.EventKey {
	if err := clipboardWrite(v.content); err != nil {
		v.app.Flash().Err(err)
		return nil
	}
	v.app.Flash().Info("Content copied to clipboard")
	return nil
}

func (v *ArtifactViewer) saveCmd(*tcell.EventKey) *tcell.EventKey {
	homeDir, _ := os.UserHomeDir()
	path := filepath.Join(homeDir, v.filename)
	
	if err := os.WriteFile(path, []byte(v.content), 0644); err != nil {
		v.app.Flash().Err(err)
		return nil
	}
	v.app.Flash().Info(fmt.Sprintf("Saved to %s", path))
	return nil
}

func (v *ArtifactViewer) backCmd(*tcell.EventKey) *tcell.EventKey {
	if v.app.Content.CanPop() {
		v.app.Content.Pop()
	}
	return nil
}
