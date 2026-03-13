package quantinuum

// loginRequest is the JSON body for POST /login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginResponse is returned by POST /login.
type loginResponse struct {
	IDToken      string `json:"id-token"`
	RefreshToken string `json:"refresh-token"`
}

// jobRequest is the JSON body for POST /job.
type jobRequest struct {
	Machine  string `json:"machine"`
	Language string `json:"language"`
	Program  string `json:"program"`
	Count    int    `json:"count"`
	Name     string `json:"name,omitempty"`
}

// jobResponse is returned by POST /job.
type jobResponse struct {
	Job string `json:"job"`
}

// jobStatusResponse is returned by GET /job/{id}.
type jobStatusResponse struct {
	Job     string         `json:"job"`
	Status  string         `json:"status"`
	Results map[string]int `json:"results,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// apiErrorResponse is the standard error response format.
type apiErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
