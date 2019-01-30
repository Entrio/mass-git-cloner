# Git Cloner
#### Simple utility that helps to mass clone all of user's repos from bitbucket server
**About:**
If you ever run into a situation where you need to clone all of the repositories on a given bitbucket server and there are too many to do so manually, then this little tool is for you.

**Usage instructions:**
This tool expects few environmental variables to be set in order to work.

***IMPORTANT*** it is assumed that the repositories are available for ssh cloning *AND* that current user has a public/private key setup with the git server. If repos are available only via http, create an issue. If there is demand, I will implement it, but right now, you are out of luck. Sorry!

###### ENV Vars
Variable | Description
------------ | -------------
BASE_BB_URL | bitbucket API service should be reachable at this end. This is usually `https://some.bitbucketserver.com/rest/api/1.0/projects`
BASE_BB_REPO_URL | Url where repositories for a project prefix can be found. Add `%s` where the project key should go. ***Example***: if your BASE_BB_URL is `https://some.bitbucketserver.com/rest/api/1.0/projects` then your BASE_BB_REPO_URL should be `https://some.bitbucketserver.com/rest/api/1.0/projects/%s/repos`. If you do not specify a value for this variable, then the system will use default format: `BASE_BB_URL/%s/repos`
BB_USERNAME | Bitbucket username, can be left blank if the server does not require login.
BB_PASSWORD | Bitbucket password. The username and password are used to attach Basic Authorization to each request. Can be left blank
BB_GIT_BASE_FOLDER | base folder where the projects and repos should be cloned to. ***Default***: ./

Once started, the application will attempt to get a list of all projects and a list of repositories for each project. Once the list os formed, the application will attempt to create a directory (project name) if it doesnt exist and then proceed to clone all of the repositories.

***Note:*** This utility clones repositories by executing `git clone` command. If git is not installed on your system or is not available in your shell then this application will fail.


