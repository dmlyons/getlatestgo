// Command getlatestgo-gui is the desktop (Fyne) front end of getlatestgo.
package main

import (
	_ "embed"
	"fmt"
	goversion "go/version"
	"net/url"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/dmlyons/getlatestgo/download"
	installpkg "github.com/dmlyons/getlatestgo/install"
	"github.com/dmlyons/getlatestgo/releases"
)

const appID = "io.github.dmlyons.getlatestgo"

// icon is the Go gopher by Renée French, licensed CC BY 4.0
// (https://go.dev/blog/gopher), from golang/website _content/doc/gopher/gophercolor.png.
//
//go:embed icon.png
var icon []byte

func main() {
	a := app.NewWithID(appID)
	a.SetIcon(fyne.NewStaticResource("icon.png", icon))
	w := a.NewWindow("getlatestgo")
	g := newGUI(w, download.NewRetryClient())
	w.SetContent(g.content())
	w.Resize(fyne.NewSize(720, 0))
	g.refresh()
	w.ShowAndRun()
}

// gui holds the window state. All fields are only touched on the Fyne UI
// goroutine; network, disk, and install work runs in background goroutines
// that hand results back via fyne.Do.
type gui struct {
	win    fyne.Window
	client *download.RetryClient

	releaseList []releases.GoRelease
	latest      string           // newest stable version in releaseList
	file        *releases.GoFile // file for the selected version and this host

	form        *widget.Form // built by content; must exist before refresh
	installed   *widget.Label
	version     *widget.Select
	filename    *widget.Label
	link        *widget.Hyperlink
	sha256      *widget.Label
	progress    *widget.ProgressBar
	status      *widget.Label
	refreshBtn  *widget.Button
	downloadBtn *widget.Button
	installBtn  *widget.Button
}

func newGUI(win fyne.Window, client *download.RetryClient) *gui {
	g := &gui{
		win:       win,
		client:    client,
		installed: &widget.Label{Wrapping: fyne.TextWrapWord},
		filename:  widget.NewLabel(""),
		link:      widget.NewHyperlink("", nil),
		sha256:    &widget.Label{Selectable: true, Truncation: fyne.TextTruncateEllipsis},
		progress:  widget.NewProgressBar(),
		status:    &widget.Label{Wrapping: fyne.TextWrapWord},
	}
	g.version = widget.NewSelect(nil, g.selectVersion)
	g.version.PlaceHolder = "Loading releases…"
	g.refreshBtn = widget.NewButton("Refresh", g.refresh)
	g.downloadBtn = widget.NewButton("Download", func() { g.fetch(false) })
	g.installBtn = widget.NewButton("Download & Install", g.confirmInstall)
	g.installBtn.Importance = widget.HighImportance
	return g
}

func (g *gui) content() fyne.CanvasObject {
	g.form = widget.NewForm(
		widget.NewFormItem("Installed", g.installed),
		widget.NewFormItem("Version", g.version),
		widget.NewFormItem("Platform", widget.NewLabel(runtime.GOOS+"/"+runtime.GOARCH)),
		widget.NewFormItem("File", g.filename),
		widget.NewFormItem("URL", g.link),
		widget.NewFormItem("SHA256", g.sha256),
	)
	buttons := container.NewHBox(g.refreshBtn, layout.NewSpacer(), g.downloadBtn, g.installBtn)
	return container.NewVBox(g.form, g.progress, g.status, buttons)
}

// setBusy disables the controls while a background operation runs.
func (g *gui) setBusy(busy bool) {
	for _, d := range []fyne.Disableable{g.version, g.refreshBtn} {
		if busy {
			d.Disable()
		} else {
			d.Enable()
		}
	}
	g.form.Refresh() // re-style the labels of re-enabled/disabled form fields
	g.updateActions(busy)
}

// updateActions enables Download/Install only when a file is resolved.
func (g *gui) updateActions(busy bool) {
	if busy || g.file == nil {
		g.downloadBtn.Disable()
		g.installBtn.Disable()
		return
	}
	g.downloadBtn.Enable()
	if installpkg.GraphicalSupported() {
		g.installBtn.Enable()
	} else {
		g.installBtn.Disable()
	}
}

func (g *gui) refresh() {
	g.setBusy(true)
	g.showInstalled()
	g.status.SetText("Fetching release list…")
	go func() {
		list, err := releases.Fetch(g.client, releases.DefaultURL)
		fyne.Do(func() {
			defer g.setBusy(false)
			if err != nil {
				g.status.SetText("Could not fetch the release list: " + err.Error())
				return
			}
			g.releaseList = list
			versions := releases.StableVersions(list)
			g.version.PlaceHolder = "Select a version"
			g.version.SetOptions(versions)
			g.status.SetText("")
			g.latest = ""
			if len(versions) > 0 {
				g.latest = versions[0]
			}
			g.showInstalled()
			if len(versions) == 0 {
				g.status.SetText("No stable releases found.")
				return
			}
			g.version.SetSelectedIndex(0) // newest stable, like the CLI default
		})
	}()
}

// showInstalled describes the Go installation at installpkg.GoRoot and how it
// compares to the newest stable release (once the release list is loaded).
func (g *gui) showInstalled() {
	inst, err := installpkg.Installed(installpkg.GoRoot)
	switch {
	case err != nil:
		g.installed.Importance = widget.DangerImportance
		g.installed.SetText(err.Error())
		return
	case inst == nil:
		g.installed.Importance = widget.MediumImportance
		g.installed.SetText("Go is not installed in " + installpkg.GoRoot)
		return
	}

	text := inst.Version
	if !inst.Time.IsZero() {
		text += " (" + inst.Time.Format("2006-01-02") + ")"
	}
	text += " in " + installpkg.GoRoot
	g.installed.Importance = widget.MediumImportance
	if g.latest != "" && goversion.IsValid(inst.Version) && goversion.IsValid(g.latest) {
		switch c := goversion.Compare(inst.Version, g.latest); {
		case c < 0:
			text += " — update available: " + g.latest
			g.installed.Importance = widget.WarningImportance
		case c == 0:
			text += " — up to date"
			g.installed.Importance = widget.SuccessImportance
		default:
			text += " — newer than the latest stable release (" + g.latest + ")"
		}
	}
	g.installed.SetText(text)
}

func (g *gui) selectVersion(version string) {
	g.file = nil
	g.filename.SetText("")
	g.link.SetText("")
	g.link.SetURL(nil)
	g.sha256.SetText("")
	g.progress.SetValue(0)
	defer g.updateActions(false)

	release, err := releases.FindRelease(g.releaseList, version)
	if err != nil {
		g.status.SetText(err.Error())
		return
	}
	f, err := releases.FindFile(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		g.status.SetText(err.Error())
		return
	}

	g.file = f
	g.filename.SetText(f.Filename)
	u, err := url.Parse(f.DownloadURL())
	if err == nil {
		g.link.SetURL(u)
	}
	g.link.SetText(f.DownloadURL())
	g.sha256.SetText(f.Sha256)
	g.status.SetText("")
}

func (g *gui) confirmInstall() {
	current := installpkg.GoRoot
	if inst, err := installpkg.Installed(installpkg.GoRoot); err == nil && inst != nil {
		current = inst.Version + " in " + installpkg.GoRoot
	}
	msg := fmt.Sprintf("This replaces %s with %s.\nYou will be asked for an administrator password.", current, g.file.Version)
	dialog.ShowConfirm("Install "+g.file.Version+"?", msg, func(ok bool) {
		if ok {
			g.fetch(true)
		}
	}, g.win)
}

// fetch downloads (or reuses a verified cached copy of) the selected file and,
// if install is set, installs it.
func (g *gui) fetch(install bool) {
	f := *g.file
	g.setBusy(true)
	g.progress.SetValue(0)
	g.status.SetText("Downloading " + f.Filename + "…")

	go func() {
		localFile := f.LocalPath()
		cached, err := download.EnsureFile(g.client, localFile, f.DownloadURL(), f.Sha256, g.progressReporter(int64(f.Size)))
		if err != nil {
			fyne.Do(func() { g.fail(err) })
			return
		}

		done := "Downloaded and verified (SHA256 OK): " + localFile
		if cached {
			done = "Using cached local download (SHA256 OK): " + localFile
		}
		if !install {
			fyne.Do(func() {
				g.setBusy(false)
				g.progress.SetValue(1)
				g.status.SetText(done)
			})
			return
		}
		fyne.Do(func() {
			g.progress.SetValue(1)
			g.status.SetText(done + "\nInstalling…")
		})

		if err := installpkg.InstallGoGraphical(localFile); err != nil {
			fyne.Do(func() { g.fail(fmt.Errorf("install failed: %w", err)) })
			return
		}
		fyne.Do(func() {
			g.setBusy(false)
			g.showInstalled()
			msg := f.Version + " installed to " + installpkg.GoRoot
			g.status.SetText(msg)
			dialog.ShowInformation("Installed", msg, g.win)
		})
	}()
}

func (g *gui) fail(err error) {
	g.setBusy(false)
	g.status.SetText(err.Error())
	dialog.ShowError(err, g.win)
}

// progressReporter feeds download progress to the progress bar, falling back
// to the feed's file size when the server sends no Content-Length. It only
// schedules a UI update when the bar would visibly move.
func (g *gui) progressReporter(feedSize int64) download.Progress {
	last := -1
	return func(downloaded, total int64) {
		if total <= 0 {
			total = feedSize
		}
		if total <= 0 {
			return
		}
		permille := int(downloaded * 1000 / total)
		if permille == last {
			return
		}
		last = permille
		fyne.Do(func() { g.progress.SetValue(float64(permille) / 1000) })
	}
}
