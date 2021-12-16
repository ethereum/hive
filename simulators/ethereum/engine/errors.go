package main

// Placeholder for method that will verify correct errors for each of the clients
func checkError(t *TestEnv, error_name string, err error) (string, bool) {
	if err == nil {
		return "No error", true
	}
	return "ok", false
}
