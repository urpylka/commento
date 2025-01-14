package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/http"
)

func githubGetPrimaryEmail(accessToken string) (string, error) {

	client := &http.Client{}
	request, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)

	if err != nil {
		logger.Errorf("error in getting email: %v", err)
	}

	request.Header.Set("Authorization", "token " + accessToken)
	resp, err := client.Do(request)

	if err != nil {
		logger.Errorf("error in getting email: %v", err)
	}

	defer resp.Body.Close()

	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errorCannotReadResponse
	}

	user := []map[string]interface{}{}
	if err := json.Unmarshal(contents, &user); err != nil {
		logger.Errorf("error unmarshaling github user: %v", err)
		logger.Errorf("resp: %v", contents)
		return "", errorInternal
	}

	nonPrimaryEmail := ""
	for _, email := range user {
		nonPrimaryEmail = email["email"].(string)
		if email["primary"].(bool) {
			return email["email"].(string), nil
		}
	}

	return nonPrimaryEmail, nil
}

func githubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	commenterToken := r.FormValue("state")
	code := r.FormValue("code")

	_, err := commenterGetByCommenterToken(commenterToken)
	if err != nil && err != errorNoSuchToken {
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}

	token, err := githubConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		fmt.Fprintf(w, "Error: %s", err.Error())
		return
	}

	email, err := githubGetPrimaryEmail(token.AccessToken)
	if err != nil {
		fmt.Fprintf(w, "Error: %s", err.Error())
		return
	}

	client := &http.Client{}
	request, err := http.NewRequest("GET", "https://api.github.com/user", nil)

	if err != nil {
		logger.Errorf("error in callback handler: %v", err)
	}

	request.Header.Set("Authorization", "token " + token.AccessToken)
	resp, err := client.Do(request)

	if err != nil {
		fmt.Fprintf(w, "Error: %s", err.Error())
		return
	}
	defer resp.Body.Close()

	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(w, "Error: %s", errorCannotReadResponse.Error())
		return
	}

	user := make(map[string]interface{})
	if err := json.Unmarshal(contents, &user); err != nil {
		fmt.Fprintf(w, "Error: %s", errorInternal.Error())
		return
	}

	if email == "" {
		if user["email"] == nil {
			fmt.Fprintf(w, "Error: no email address returned by Github")
			return
		}

		email = user["email"].(string)
	}

	name := user["login"].(string)
	if user["name"] != nil {
		name = user["name"].(string)
	}

	link := "undefined"
	if user["html_url"] != nil {
		link = user["html_url"].(string)
	}

	photo := "undefined"
	if user["avatar_url"] != nil {
		photo = user["avatar_url"].(string)
	}

	c, err := commenterGetByEmail("github", email)
	if err != nil && err != errorNoSuchCommenter {
		fmt.Fprintf(w, "Error: %s", err.Error())
		return
	}

	var commenterHex string

	if err == errorNoSuchCommenter {
		commenterHex, err = commenterNew(email, name, link, photo, "github", "")
		if err != nil {
			fmt.Fprintf(w, "Error: %s", err.Error())
			return
		}
	} else {
		if err = commenterUpdate(c.CommenterHex, email, name, link, photo, "github"); err != nil {
			logger.Warningf("cannot update commenter: %s", err)
			// not a serious enough to exit with an error
		}

		commenterHex = c.CommenterHex
	}

	if err := commenterSessionUpdate(commenterToken, commenterHex); err != nil {
		fmt.Fprintf(w, "Error: %s", err.Error())
		return
	}

	fmt.Fprintf(w, "<html><script>window.parent.close()</script></html>")
}
