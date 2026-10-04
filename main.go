package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"filelist/internal/comm"
	"filelist/internal/door32"
	"filelist/internal/elebbs"
	"filelist/internal/ui"
)

func main() {
	os.Exit(run())
}

func run() int {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)

	local := flag.Bool("local", false, "use local console instead of inherited socket")
	ws := flag.Bool("ws", false, "treat inherited socket as gorilla websocket frames")
	sysHint := flag.String("sys", "", "EleBBS system path (default: %RA%)")
	flag.Parse()

	viewCmd, extra := elebbs.LoadINI(exeDir)
	if *sysHint == "" {
		*sysHint = extra["syspath"]
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	drop := &door32.Drop{Local: true, NodePath: cwd, HandleName: "Local"}
	if found, err := door32.Find(""); err == nil {
		if parsed, err := door32.Parse(found); err == nil && parsed != nil {
			drop = parsed
			drop.NodePath = cwd
			if *local {
				drop.Local = true
			}
		}
	} else if !*local {
		fmt.Fprintf(os.Stderr, "door32.sys: %v\n", err)
		return 1
	}

	sysPath := elebbs.FindSysPath(*sysHint, drop.NodePath, exeDir)

	userRec := drop.UserRec
	if userRec <= 0 {
		if n := elebbs.ParseDoorSysUserRec(drop.NodePath); n > 0 {
			userRec = n
		}
	}

	sess := elebbs.ResolveSession(elebbs.DropHint{
		NodePath: drop.NodePath,
		SysPath:  sysPath,
		UserName: drop.RealName,
		Handle:   drop.HandleName,
		UserRec:  userRec,
	})
	if sess.FileArea == 0 {
		fail := "could not read current file area from EXITINFO.BBS or USERS.BBS"
		if *local {
			fmt.Fprintln(os.Stderr, fail)
			fmt.Fprintf(os.Stderr, "sys=%s node=%s\n", sysPath, drop.NodePath)
			return 1
		}
		return doorError(drop, *ws, *local, fail)
	}

	ctx, err := elebbs.LoadArea(sysPath, drop.NodePath, sess.FileArea)
	if err != nil {
		fail := fmt.Sprintf("file area %d: %v", sess.FileArea, err)
		if *local {
			fmt.Fprintln(os.Stderr, fail)
			fmt.Fprintf(os.Stderr, "sys=%s node=%s area-src=%s group-src=%s group=%d\n",
				sysPath, drop.NodePath, sess.AreaSource, sess.GroupSource, sess.FileGroup)
			return 1
		}
		return doorError(drop, *ws, *local, fail)
	}
	if sess.FileGroup != 0 {
		if g := elebbs.LookupGroup(sysPath, sess.FileGroup); g.AreaNum != 0 || g.Name != "" {
			ctx.Group = g
		}
	}

	tags, _ := elebbs.LoadTagList(drop.NodePath)

	stream, closer, err := openStream(drop, *local, *ws)
	if err != nil {
		fmt.Fprintf(os.Stderr, "socket: %v\n", err)
		return 1
	}
	defer closer()

	user := drop.HandleName
	if user == "" {
		user = drop.RealName
	}
	if drop.TimeLeft > 0 {
		user = fmt.Sprintf("%s %dm", user, drop.TimeLeft)
	}

	app := &ui.App{
		Out:      stream,
		Keys:     comm.NewKeyboard(stream),
		Ctx:      ctx,
		Tags:     tags,
		ViewCmd:  viewCmd,
		Node:     drop.NodePath,
		NodeNum:  drop.Node,
		Handle:   drop.Handle,
		User:     user,
		Security: drop.Security,
	}
	if err := ui.Run(app); err != nil {
		return 0
	}
	return 0
}

func openStream(drop *door32.Drop, local, websocketMode bool) (comm.Stream, func(), error) {
	if local || drop.Local {
		s := comm.LocalStdio()
		return s, func() { _ = s.Close() }, nil
	}
	s, err := comm.FromDoorHandle(drop.Handle)
	if err != nil {
		return nil, nil, err
	}
	comm.HideConsole()
	return s, func() {}, nil
}

func doorError(drop *door32.Drop, ws, local bool, msg string) int {
	stream, closer, err := openStream(drop, local, ws)
	if err != nil {
		fmt.Fprintln(os.Stderr, msg)
		return 1
	}
	defer closer()
	_, _ = fmt.Fprintf(stream, "\r\n%s\r\nPress a key to return...\r\n", msg)
	k := comm.NewKeyboard(stream)
	_, _ = k.Next(0)
	return 1
}
