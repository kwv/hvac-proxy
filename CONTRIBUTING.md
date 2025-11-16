# Contributing to **hvac-proxy**

Thanks for your interest in contributing to **hvac-proxy**!\
This guide explains how to develop, build, test, and release new
versions of the project.

------------------------------------------------------------------------

## 🛠️ Development Setup

### 1. Clone the Repository

``` sh
git clone https://github.com/kwv4/hvac-proxy.git
cd hvac-proxy
```

### 2. Build the Docker Image Locally

``` sh
make build
```

This uses the current Git tag (e.g., `v1.2.3`) to version the image.

------------------------------------------------------------------------

## 🚀 Releasing a New Version

The project uses **Git tags** to trigger automated Docker image builds
and releases.

### Steps to Publish a New Version

1.  Ensure your changes are committed to `main`.
2.  Tag the commit with a semantic version:

``` sh
git tag v1.2.3
git push origin v1.2.3
```

3.  (Optional) Run the release process locally:

``` sh
export DOCKERHUB_PASSWORD=your-dockerhub-token
make release
```

This will:

-   Build the Docker image\
-   Tag it as `kwv4/hvac-proxy:<version>` and `latest`\
-   Push both tags to Docker Hub\
-   Clean up local images

> 🔐 **Note:** You must have a valid Docker Hub access token set as
> `DOCKERHUB_PASSWORD`.

------------------------------------------------------------------------

## 🧪 Testing Locally

To test the image before publishing:

``` sh
docker build -t hvac-proxy:test .
docker run --rm -p 8080:8080 hvac-proxy:test
```
------------------------------------------------------------------------

## 🔄 Contribution Workflow

Create a branch for your changes (e.g., feature/your-feature-name or bugfix/your-bug-name).
Commit your changes to this branch.
Open a pull request (PR) targeting the main branch.
Ensure your PR includes a clear description of the changes.
Add any relevant labels or assignees as needed.

------------------------------------------------------------------------

## 🤖 CI/CD Automation

GitHub Actions automatically builds and pushes Docker images whenever a
**Git tag** (e.g., `v1.2.3`) is pushed.

The workflow:

-   Extracts the version from the tag\
-   Builds the Docker image\
-   Pushes it to Docker Hub as:
    -   `kwv4/hvac-proxy:<version>`
    -   `kwv4/hvac-proxy:latest`

------------------------------------------------------------------------

## 🧩 Required Secrets for CI

Add these secrets in your GitHub repository settings:

  Secret Name            Value
  ---------------------- ---------------------------
  `DOCKERHUB_USERNAME`   `kwv4`
  `DOCKERHUB_TOKEN`      *Docker Hub access token*

------------------------------------------------------------------------

## 💬 Questions?

Feel free to open an issue or start a discussion if you have questions,
improvement ideas, or suggestions.\
We welcome contributions of all kinds!
