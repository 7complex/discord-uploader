package main

import "os/exec"

// checkDependencies retorna as ferramentas ausentes no PATH do host.
func checkDependencies() []string {
	ferramentas := []string{"bash", "curl", "find", "stat", "awk", "sed", "grep", "date"}
	var faltantes []string
	for _, f := range ferramentas {
		if _, err := exec.LookPath(f); err != nil {
			faltantes = append(faltantes, f)
		}
	}
	return faltantes
}
