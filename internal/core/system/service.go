package system

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var (
	ErrDisallowedService = errors.New("service name is not in the allowed whitelist")
	ErrDisallowedAction  = errors.New("service action is not in the allowed whitelist")
)

// AllowedServices whitelist to prevent arbitrary command execution via systemctl
var AllowedServices = map[string]bool{
	"nginx":       true,
	"apache2":     true,
	"lsws":        true,
	"caddy":       true,
	"mariadb":     true,
	"mysql":       true,
	"postgresql":  true,
	"php7.4-fpm":  true,
	"php8.0-fpm":  true,
	"php8.1-fpm":  true,
	"php8.2-fpm":  true,
	"php8.3-fpm":  true,
	"php8.4-fpm":  true,
	"php8.5-fpm":  true,
	"seintech":    true,
}

var AllowedActions = map[string]bool{
	"start":     true,
	"stop":      true,
	"restart":   true,
	"reload":    true,
	"status":    true,
	"is-active": true,
	"enable":    true,
	"disable":   true,
}

type ServiceState struct {
	Name    string `json:"name"`
	Active  bool   `json:"active"`
	Status  string `json:"status"` // running, stopped, failed, unknown
	Uptime  string `json:"uptime,omitempty"`
}

type ServiceManager struct{}

func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}

// ExecuteSystemctl executes systemctl safely with strict whitelist validation and timeout.
func (sm *ServiceManager) ExecuteSystemctl(ctx context.Context, action, serviceName string) (string, error) {
	if !AllowedActions[action] {
		return "", fmt.Errorf("%w: %s", ErrDisallowedAction, action)
	}

	if !AllowedServices[serviceName] {
		return "", fmt.Errorf("%w: %s", ErrDisallowedService, serviceName)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", action, serviceName)
	output, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(output))

	if err != nil {
		// systemctl is-active returns exit code 3 or 4 if inactive/failed
		if action == "is-active" {
			return outStr, nil
		}
		return outStr, fmt.Errorf("systemctl %s %s failed: %w (%s)", action, serviceName, err, outStr)
	}

	return outStr, nil
}

// GetStatus checks if the service is currently running.
func (sm *ServiceManager) GetStatus(ctx context.Context, serviceName string) (ServiceState, error) {
	out, err := sm.ExecuteSystemctl(ctx, "is-active", serviceName)
	state := ServiceState{
		Name:   serviceName,
		Status: out,
		Active: out == "active",
	}
	return state, err
}

func (sm *ServiceManager) Start(ctx context.Context, serviceName string) error {
	_, err := sm.ExecuteSystemctl(ctx, "start", serviceName)
	return err
}

func (sm *ServiceManager) Stop(ctx context.Context, serviceName string) error {
	_, err := sm.ExecuteSystemctl(ctx, "stop", serviceName)
	return err
}

func (sm *ServiceManager) Restart(ctx context.Context, serviceName string) error {
	_, err := sm.ExecuteSystemctl(ctx, "restart", serviceName)
	return err
}

func (sm *ServiceManager) Reload(ctx context.Context, serviceName string) error {
	_, err := sm.ExecuteSystemctl(ctx, "reload", serviceName)
	return err
}
