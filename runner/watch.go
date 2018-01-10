package runner

import (
	"fmt"
	"strings"

	"github.com/yext/edward/home"

	"github.com/pkg/errors"
	"github.com/yext/edward/builder"
	"github.com/yext/edward/services"
	"github.com/yext/edward/tracker"
	fsnotify "gopkg.in/fsnotify.v1"
)

// BeginWatch starts auto-restart watches for the provided services. The function returned will close the
// watcher.
func BeginWatch(dirConfig *home.EdwardConfiguration, service services.ServiceOrGroup, restart func() error, logger Logger) (func(), error) {
	watches, err := service.Watch()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(watches) == 0 {
		return nil, nil
	}

	var watchers []*fsnotify.Watcher
	for _, watch := range watches {
		watcher, err := startWatch(dirConfig, &watch, restart, logger)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		watchers = append(watchers, watcher)
	}

	closeAll := func() {
		logger.Printf("Closing watchers")
		for _, watch := range watchers {
			watch.Close()
		}
	}
	return closeAll, nil
}

func startWatch(dirConfig *home.EdwardConfiguration, watch *services.ServiceWatch, restart func() error, logger Logger) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		for event := range watcher.Events {
			if event.Op == fsnotify.Write {
				var wasExcluded bool
				for _, excluded := range watch.ExcludedPaths {
					if strings.HasPrefix(event.Name, excluded) {
						logger.Printf("File is under excluded path: %v\n", excluded)
						wasExcluded = true
						break
					}
				}

				if wasExcluded {
					continue
				}
				fmt.Printf("Rebuilding %v\n", watch.Service.GetName())
				err = rebuildService(dirConfig, watch.Service, restart, logger)
				if err != nil {
					logger.Printf("Could not rebuild %v: %v\n", watch.Service.GetName(), err)
				}
			}
		}
		logger.Printf("No more events in watcher")
	}()

	for _, dir := range watch.IncludedPaths {
		logger.Printf("Adding: %s\n", dir)
		err = watcher.Add(dir)
		if err != nil {
			logger.Printf("%v\n", err)
			watcher.Close()
			return nil, errors.WithStack(err)
		}
	}
	return watcher, nil
}

func rebuildService(dirConfig *home.EdwardConfiguration, service *services.ServiceConfig, restart func() error, logger Logger) error {
	b := builder.New(services.OperationConfig{}, services.ContextOverride{})
	err := b.BuildWithTracker(dirConfig, tracker.NewTask(func(updatedTask tracker.Task) {}), service, true)
	if err != nil {
		return fmt.Errorf("build failed: %v", err)
	}
	logger.Printf("Build suceeded\n")
	err = restart()
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
