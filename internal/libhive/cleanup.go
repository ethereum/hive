package libhive

import (
	"context"
	"fmt"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

// CleanupOptions configures cleanup behavior
type CleanupOptions struct {
	InstanceID    string        // Clean specific instance, empty for all
	OlderThan     time.Duration // Clean containers older than duration
	DryRun        bool          // Show what would be cleaned without doing it
	ContainerType string        // Filter by container type (client, simulator, proxy)
}

// CleanupHiveContainers finds and removes Hive containers based on labels
func CleanupHiveContainers(ctx context.Context, client *docker.Client, opts CleanupOptions) error {
	// Build label filter
	filters := map[string][]string{
		"label": {"hive.instance"}, // All containers with hive.instance label
	}

	if opts.InstanceID != "" {
		filters["label"] = append(filters["label"], "hive.instance="+opts.InstanceID)
	}

	if opts.ContainerType != "" {
		filters["label"] = append(filters["label"], "hive.type="+opts.ContainerType)
	}

	containers, err := client.ListContainers(docker.ListContainersOptions{
		Context: ctx,
		All:     true,
		Filters: filters,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	for _, container := range containers {
		if opts.OlderThan > 0 {
			createdTimeStr, exists := container.Labels[LabelHiveCreated]
			if !exists {
				continue // Skip containers without creation timestamp
			}
			createdTime, err := time.Parse(time.RFC3339, createdTimeStr)
			if err != nil || time.Since(createdTime) < opts.OlderThan {
				continue
			}
		}

		containerType := container.Labels[LabelHiveType]
		if containerType == "" {
			containerType = "unknown"
		}

		if opts.DryRun {
			fmt.Printf("Would remove container %s (%s)\n", container.ID[:12], containerType)
			continue
		}

		err := client.RemoveContainer(docker.RemoveContainerOptions{
			ID:    container.ID,
			Force: true,
		})
		if err != nil {
			fmt.Printf("Failed to remove container %s: %v\n", container.ID[:12], err)
		} else {
			fmt.Printf("Removed container %s (%s)\n", container.ID[:12], containerType)
		}
	}

	return nil
}

// ListHiveContainers lists all Hive containers with their metadata
func ListHiveContainers(ctx context.Context, client *docker.Client, instanceID string) error {
	// Build label filter
	filters := map[string][]string{
		"label": {"hive.instance"}, // All containers with hive.instance label
	}

	if instanceID != "" {
		filters["label"] = append(filters["label"], "hive.instance="+instanceID)
	}

	containers, err := client.ListContainers(docker.ListContainersOptions{
		Context: ctx,
		All:     true,
		Filters: filters,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	if len(containers) == 0 {
		fmt.Println("No Hive containers found")
		return nil
	}

	fmt.Printf("%-12s %-20s %-12s %-12s %-20s %-10s %s\n", 
		"CONTAINER ID", "NAME", "TYPE", "STATUS", "CREATED", "INSTANCE", "DETAILS")
	fmt.Println(strings.Repeat("-", 120))

	for _, container := range containers {
		containerType := container.Labels[LabelHiveType]
		if containerType == "" {
			containerType = "unknown"
		}

		instanceID := container.Labels[LabelHiveInstance]
		if len(instanceID) > 10 {
			instanceID = instanceID[:10] + "..."
		}

		createdStr := container.Labels[LabelHiveCreated]
		if createdStr != "" {
			if createdTime, err := time.Parse(time.RFC3339, createdStr); err == nil {
				createdStr = createdTime.Format("2006-01-02 15:04")
			}
		}

		details := ""
		switch containerType {
		case ContainerTypeClient:
			if clientName := container.Labels[LabelHiveClientName]; clientName != "" {
				details = "client:" + clientName
			}
		case ContainerTypeSimulator:
			if simName := container.Labels[LabelHiveSimulator]; simName != "" {
				details = "sim:" + simName
			}
		case ContainerTypeProxy:
			details = "hiveproxy"
		}

		containerName := ""
		if len(container.Names) > 0 {
			containerName = container.Names[0]
			// Remove the leading "/" that Docker adds to container names
			if strings.HasPrefix(containerName, "/") {
				containerName = containerName[1:]
			}
			// Truncate long names
			if len(containerName) > 18 {
				containerName = containerName[:18] + "..."
			}
		}

		fmt.Printf("%-12s %-20s %-12s %-12s %-20s %-10s %s\n",
			container.ID[:12], containerName, containerType, container.Status, createdStr, instanceID, details)
	}

	return nil
}