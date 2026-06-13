// Package worker assembles background runtime workers.
package worker

import kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"

// Build assembles the background runtime worker from runtime worker options.
func Build(options kgruntime.WorkerOptions) kgruntime.Runner {
	return kgruntime.NewWorker(options)
}
