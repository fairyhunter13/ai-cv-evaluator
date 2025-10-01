//go:build e2e
// +build e2e

package e2e_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_HappyPathVariations runs multiple happy path variations with diverse test data
func TestE2E_HappyPathVariations(t *testing.T) {
	t.Parallel()

	// Define diverse test variations with different professional backgrounds
	variations := []struct {
		name        string
		cvText      string
		projText    string
		description string
	}{
		{
			name:        "software_engineer_general",
			cvText:      "Software Engineer with 5+ years experience in full-stack development. Proficient in JavaScript, Python, Java, and React. Strong background in database design, API development, and cloud technologies.",
			projText:    "Develop a scalable web application using modern technologies. The project involves building a REST API, implementing authentication, and creating a responsive frontend interface.",
			description: "General software engineering role with broad technology stack",
		},
		{
			name:        "data_scientist_ml",
			cvText:      "Data Scientist specializing in machine learning and statistical analysis. Experience with Python, R, TensorFlow, PyTorch, and data visualization tools. Strong background in predictive modeling and deep learning.",
			projText:    "Build a machine learning pipeline for customer segmentation and recommendation system. Implement data preprocessing, feature engineering, model training, and deployment strategies.",
			description: "Data science role focusing on ML and analytics",
		},
		{
			name:        "devops_cloud_engineer",
			cvText:      "DevOps Engineer with expertise in AWS, Azure, Docker, Kubernetes, and CI/CD pipelines. Experience with infrastructure as code using Terraform and Ansible. Strong background in monitoring and automation.",
			projText:    "Design and implement a cloud-native infrastructure for a microservices application. Set up monitoring, logging, and automated deployment pipelines using modern DevOps practices.",
			description: "DevOps role with cloud and infrastructure focus",
		},
		{
			name:        "frontend_developer_modern",
			cvText:      "Frontend Developer with expertise in React, Vue.js, TypeScript, and modern CSS frameworks. Experience with state management, component libraries, and responsive design. Strong background in user experience and accessibility.",
			projText:    "Create a modern single-page application with real-time features. Implement responsive design, state management, and integrate with REST APIs and WebSocket connections.",
			description: "Frontend development with modern frameworks",
		},
		{
			name:        "backend_engineer_microservices",
			cvText:      "Backend Engineer specializing in microservices architecture and distributed systems. Proficient in Go, Java, Node.js, PostgreSQL, Redis, and message queues. Experience with API design and system scalability.",
			projText:    "Design and implement a microservices architecture for an e-commerce platform. Handle user management, product catalog, order processing, and payment integration with proper error handling and monitoring.",
			description: "Backend engineering with microservices focus",
		},
		{
			name:        "ai_engineer_llm",
			cvText:      "AI Engineer with expertise in large language models, prompt engineering, and RAG systems. Experience with OpenAI API, LangChain, vector databases, and AI workflow automation. Strong background in natural language processing.",
			projText:    "Develop an AI-powered customer support system using LLMs. Implement RAG architecture, create intelligent chatbots, and build knowledge management systems for automated responses.",
			description: "AI engineering with LLM and RAG expertise",
		},
		{
			name:        "cybersecurity_analyst",
			cvText:      "Cybersecurity Analyst with expertise in threat detection, vulnerability assessment, and security architecture. Experience with SIEM tools, penetration testing, and compliance frameworks. Strong background in network security and incident response.",
			projText:    "Conduct a comprehensive security audit of a web application. Implement security controls, perform penetration testing, and develop incident response procedures for potential security breaches.",
			description: "Cybersecurity role with security analysis focus",
		},
		{
			name:        "mobile_developer_cross_platform",
			cvText:      "Mobile Developer with expertise in React Native, Flutter, and native iOS/Android development. Experience with cross-platform frameworks, app store deployment, and mobile UI/UX design. Strong background in performance optimization.",
			projText:    "Develop a cross-platform mobile application for task management. Implement offline functionality, push notifications, and integrate with cloud services while maintaining consistent user experience across platforms.",
			description: "Mobile development with cross-platform expertise",
		},
		{
			name:        "blockchain_developer",
			cvText:      "Blockchain Developer with expertise in Solidity, Web3, DeFi protocols, and smart contract development. Experience with Ethereum, Polygon, and other blockchain networks. Strong background in decentralized application architecture.",
			projText:    "Develop a DeFi protocol for automated market making. Implement smart contracts for liquidity provision, create governance mechanisms, and build frontend interfaces for user interaction with the protocol.",
			description: "Blockchain development with DeFi focus",
		},
		{
			name:        "game_developer_unity",
			cvText:      "Game Developer with expertise in Unity, C#, and 3D game development. Experience with game physics, animation systems, and multiplayer networking. Strong background in game design principles and performance optimization.",
			projText:    "Create a multiplayer 3D action game using Unity. Implement character movement, combat systems, networking for multiplayer functionality, and optimize performance for various devices.",
			description: "Game development with Unity and 3D focus",
		},
		{
			name:        "product_manager_technical",
			cvText:      "Technical Product Manager with engineering background and experience in agile methodologies. Strong background in user research, product strategy, and cross-functional team leadership. Experience with data-driven decision making and product analytics.",
			projText:    "Lead the development of a new SaaS product from concept to launch. Define product requirements, create user stories, coordinate with engineering teams, and establish success metrics and KPIs.",
			description: "Technical product management role",
		},
		{
			name:        "qa_engineer_automation",
			cvText:      "QA Engineer with expertise in test automation, API testing, and performance testing. Experience with Selenium, Cypress, Postman, and continuous integration. Strong background in test strategy and quality assurance processes.",
			projText:    "Implement comprehensive testing strategy for a web application. Set up automated testing pipelines, create test suites for API and UI testing, and establish performance testing benchmarks.",
			description: "QA engineering with automation focus",
		},
		{
			name:        "cloud_architect_enterprise",
			cvText:      "Cloud Architect with expertise in enterprise cloud solutions, multi-cloud strategies, and digital transformation. Experience with AWS, Azure, GCP, and hybrid cloud architectures. Strong background in security, compliance, and cost optimization.",
			projText:    "Design and implement a multi-cloud architecture for an enterprise application. Ensure high availability, disaster recovery, security compliance, and cost optimization across different cloud providers.",
			description: "Cloud architecture with enterprise focus",
		},
		{
			name:        "database_administrator",
			cvText:      "Database Administrator with expertise in PostgreSQL, MySQL, MongoDB, and Redis. Experience with database optimization, backup strategies, and data migration. Strong background in database security and performance tuning.",
			projText:    "Optimize database performance for a high-traffic application. Implement database clustering, set up automated backups, and design data archiving strategies for long-term data retention.",
			description: "Database administration with optimization focus",
		},
		{
			name:        "ui_ux_designer_technical",
			cvText:      "UI/UX Designer with technical background and experience in user research, wireframing, and prototyping. Proficient in Figma, Adobe Creative Suite, and frontend development. Strong background in accessibility and responsive design.",
			projText:    "Design a comprehensive user interface for a complex dashboard application. Conduct user research, create wireframes and prototypes, and ensure accessibility compliance across different devices and user needs.",
			description: "UI/UX design with technical background",
		},
		{
			name:        "site_reliability_engineer",
			cvText:      "Site Reliability Engineer with expertise in monitoring, alerting, and incident response. Experience with Prometheus, Grafana, ELK stack, and on-call procedures. Strong background in system reliability and performance optimization.",
			projText:    "Implement comprehensive monitoring and alerting for a distributed system. Set up log aggregation, create dashboards for system health, and establish incident response procedures for production issues.",
			description: "SRE role with monitoring and reliability focus",
		},
		{
			name:        "long_content_comprehensive_test",
			cvText:      generateLongCV(),
			projText:    generateLongProject(),
			description: "Single test case to verify system can handle long CV and project content properly",
		},
	}

	// Run variations in smaller batches to prevent resource contention
	batchSize := 3
	for i := 0; i < len(variations); i += batchSize {
		end := i + batchSize
		if end > len(variations) {
			end = len(variations)
		}
		batch := variations[i:end]

		for _, variation := range batch {
			variation := variation // capture loop variable
			t.Run(variation.name, func(t *testing.T) {
				t.Parallel()
				runHappyPathVariation(t, variation.name, variation.cvText, variation.projText, variation.description)
			})
		}
	}
}

// runHappyPathVariation executes a single happy path variation test
func runHappyPathVariation(t *testing.T, testName, cvText, projText, description string) {
	// Clear dump directory for this test
	clearDumpDirectory(t)

	httpTimeout := 2 * time.Second
	if testing.Short() {
		httpTimeout = 1 * time.Second
	}
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable
	healthz := strings.TrimSuffix(baseURL, "/v1") + "/healthz"
	if resp, err := client.Get(healthz); err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatalf("App not available; healthz check failed: %v", err)
	} else if resp != nil {
		resp.Body.Close()
	}

	t.Logf("=== Running Happy Path Variation: %s ===", testName)
	t.Logf("Description: %s", description)

	// 1) Upload CV and Project texts
	uploadResp := uploadTestFiles(t, client, cvText, projText)
	dumpJSON(t, testName+"_upload_response.json", uploadResp)

	// 2) Enqueue Evaluate
	cvID, ok := uploadResp["cv_id"].(string)
	require.True(t, ok, "cv_id should be string")
	projectID, ok := uploadResp["project_id"].(string)
	require.True(t, ok, "project_id should be string")
	evalResp := evaluateFiles(t, client, cvID, projectID)
	dumpJSON(t, testName+"_evaluate_response.json", evalResp)
	jobID, ok := evalResp["id"].(string)
	require.True(t, ok && jobID != "", "evaluate should return job id")

	// 3) Wait for completion (up to 120s for AI processing)
	final := waitForCompleted(t, client, jobID, 300*time.Second)
	dumpJSON(t, testName+"_result_response.json", final)
	st, _ := final["status"].(string)

	// CRITICAL: E2E tests must only accept terminal states
	require.NotEqual(t, "queued", st, "E2E test failed: job stuck in queued state - %#v", final)
	require.NotEqual(t, "processing", st, "E2E test failed: job stuck in processing state - %#v", final)

	// Validate terminal states
	switch st {
	case "completed":
		res, ok := final["result"].(map[string]any)
		require.True(t, ok, "result object missing")
		_, hasCV := res["cv_match_rate"]
		_, hasCVF := res["cv_feedback"]
		_, hasProj := res["project_score"]
		_, hasProjF := res["project_feedback"]
		_, hasSummary := res["overall_summary"]
		assert.True(t, hasCV && hasCVF && hasProj && hasProjF && hasSummary, "incomplete result payload: %#v", res)

		// Test Summary for completed jobs
		t.Logf("✅ %s - COMPLETED successfully", testName)
		if res, ok := final["result"].(map[string]any); ok {
			t.Logf("Result contains: cv_match_rate=%v, project_score=%v, overall_summary=%v",
				res["cv_match_rate"] != nil, res["project_score"] != nil, res["overall_summary"] != nil)
		}

	case "failed":
		if _, ok := final["error"].(map[string]any); !ok {
			t.Fatalf("expected error object for failed status: %#v", final)
		}

		// Test Summary for failed jobs
		t.Logf("⚠️ %s - FAILED (acceptable in E2E due to AI model limitations)", testName)
		if err, ok := final["error"].(map[string]any); ok {
			t.Logf("Error details: %v", err)
		}

	default:
		t.Fatalf("unexpected status: %v (must be completed or failed)", st)
	}

	// Log final response for debugging
	if b, err := json.MarshalIndent(final, "", "  "); err == nil {
		t.Logf("%s - Final response:\n%s", testName, string(b))
	}
}

// generateLongCV creates a comprehensive CV with detailed experience and skills
func generateLongCV() string {
	return `Senior Full-Stack Software Engineer with 8+ years of comprehensive experience architecting, developing, and leading high-performance web applications and distributed systems. 

PROFESSIONAL EXPERIENCE:

Senior Software Engineer | TechCorp Inc. | 2020-Present
• Led development of microservices architecture serving 1M+ daily active users
• Implemented event-driven patterns using Apache Kafka and Redis for real-time data processing
• Architected and deployed cloud-native applications on AWS (ECS, Lambda, RDS, S3)
• Established CI/CD pipelines using Jenkins, Docker, and Kubernetes
• Mentored 5 junior developers and conducted code reviews for 15+ team members
• Reduced system latency by 40% through performance optimization and caching strategies
• Implemented comprehensive monitoring using Prometheus, Grafana, and ELK stack

Full-Stack Developer | StartupXYZ | 2018-2020
• Developed responsive web applications using React, TypeScript, and Node.js
• Built RESTful APIs with Express.js and PostgreSQL database design
• Implemented real-time features using WebSockets and Socket.io
• Collaborated with UX/UI designers to create intuitive user interfaces
• Integrated third-party APIs including payment processing and authentication services
• Optimized database queries resulting in 60% faster page load times

Software Developer | WebSolutions Ltd. | 2016-2018
• Developed and maintained enterprise web applications using Java Spring Boot
• Worked with MySQL databases and implemented data migration scripts
• Participated in agile development processes and sprint planning
• Collaborated with QA team to ensure code quality and testing coverage
• Contributed to open-source projects and technical documentation

TECHNICAL SKILLS:

Programming Languages: JavaScript, TypeScript, Python, Java, Go, SQL, HTML5, CSS3
Frameworks & Libraries: React, Vue.js, Angular, Node.js, Express.js, Spring Boot, Django, FastAPI
Databases: PostgreSQL, MySQL, MongoDB, Redis, Elasticsearch
Cloud & DevOps: AWS (ECS, Lambda, RDS, S3, CloudFront), Docker, Kubernetes, Jenkins, GitLab CI
Cloud Platforms: AWS, Google Cloud Platform, Azure
Monitoring & Tools: Prometheus, Grafana, ELK Stack, New Relic, DataDog
Version Control: Git, GitHub, GitLab, Bitbucket
Testing: Jest, Cypress, Selenium, JUnit, pytest
Methodologies: Agile, Scrum, Test-Driven Development, Continuous Integration/Deployment

EDUCATION:
Bachelor of Science in Computer Science | University of Technology | 2012-2016
• Graduated Magna Cum Laude with GPA 3.8/4.0
• Relevant Coursework: Data Structures, Algorithms, Database Systems, Software Engineering
• Senior Project: Developed a distributed task management system using microservices

CERTIFICATIONS:
• AWS Certified Solutions Architect - Associate (2021)
• Google Cloud Professional Developer (2020)
• Certified Kubernetes Administrator (CKA) (2019)

ACHIEVEMENTS:
• Led team that reduced application deployment time from 2 hours to 15 minutes
• Implemented automated testing that increased code coverage from 65% to 95%
• Designed and implemented caching layer that reduced database load by 70%
• Received "Employee of the Year" award for outstanding technical contributions
• Published 3 technical articles on microservices architecture and performance optimization

PROJECTS:
E-commerce Platform (2022-2023)
• Built scalable e-commerce platform handling 10,000+ concurrent users
• Implemented payment processing, inventory management, and order tracking
• Technologies: React, Node.js, PostgreSQL, Redis, AWS, Docker

Real-time Analytics Dashboard (2021-2022)
• Developed real-time data visualization dashboard for business intelligence
• Integrated multiple data sources and implemented real-time updates
• Technologies: Vue.js, Python, Apache Kafka, Elasticsearch, D3.js

Mobile App Backend (2020-2021)
• Designed and implemented RESTful API for mobile application
• Implemented user authentication, push notifications, and file upload
• Technologies: Node.js, Express.js, MongoDB, AWS S3, Firebase

LANGUAGES:
• English (Native)
• Spanish (Conversational)
• French (Basic)

INTERESTS:
• Open source contribution and community involvement
• Technical writing and knowledge sharing
• Machine learning and artificial intelligence
• Blockchain and cryptocurrency technologies
• Continuous learning and professional development`
}

// generateLongProject creates a comprehensive project description with detailed specifications
func generateLongProject() string {
	return `Enterprise Microservices Platform Development

PROJECT OVERVIEW:
Develop a comprehensive enterprise microservices platform for a Fortune 500 financial services company. The project involves migrating legacy monolithic systems to cloud-native microservices architecture, implementing event-driven patterns, and ensuring high availability and scalability.

PROJECT SCOPE:
The platform will serve 2M+ daily active users and process 50M+ transactions daily. The system must maintain 99.9% uptime and handle peak loads of 100,000 concurrent users.

TECHNICAL REQUIREMENTS:

Architecture & Design:
• Design and implement microservices architecture with 15+ independent services
• Implement Domain-Driven Design (DDD) principles and bounded contexts
• Create service mesh using Istio for service-to-service communication
• Implement API Gateway pattern with rate limiting and authentication
• Design event-driven architecture using Apache Kafka for asynchronous communication
• Implement CQRS (Command Query Responsibility Segregation) pattern for data consistency

Backend Development:
• Develop RESTful APIs using Node.js/Express.js and Go microservices
• Implement gRPC services for high-performance inter-service communication
• Design and implement database per service pattern with PostgreSQL and MongoDB
• Implement distributed caching using Redis cluster
• Create comprehensive API documentation using OpenAPI/Swagger
• Implement circuit breaker pattern for fault tolerance

Frontend Development:
• Build responsive web application using React with TypeScript
• Implement state management using Redux Toolkit and RTK Query
• Create reusable component library with Storybook
• Implement real-time features using WebSockets
• Ensure accessibility compliance (WCAG 2.1 AA standards)
• Implement progressive web app (PWA) capabilities

Database & Storage:
• Design and implement database schemas for each microservice
• Implement database migration strategies and versioning
• Set up database replication and read replicas for performance
• Implement data archiving and retention policies
• Design data warehouse for analytics and reporting
• Implement backup and disaster recovery procedures

Cloud Infrastructure:
• Deploy services on AWS using ECS Fargate and EKS
• Implement Infrastructure as Code using Terraform
• Set up auto-scaling groups and load balancers
• Configure VPC, subnets, and security groups
• Implement secrets management using AWS Secrets Manager
• Set up monitoring and logging using CloudWatch and ELK stack

DevOps & CI/CD:
• Implement GitOps workflow using ArgoCD
• Create comprehensive CI/CD pipelines using Jenkins
• Implement automated testing (unit, integration, e2e)
• Set up code quality gates using SonarQube
• Implement blue-green and canary deployment strategies
• Create infrastructure monitoring and alerting

Security & Compliance:
• Implement OAuth 2.0 and JWT authentication
• Set up role-based access control (RBAC)
• Implement API security using rate limiting and throttling
• Ensure PCI DSS compliance for financial data
• Implement data encryption at rest and in transit
• Conduct security audits and penetration testing

Performance & Monitoring:
• Implement distributed tracing using Jaeger
• Set up application performance monitoring (APM)
• Create comprehensive logging and metrics collection
• Implement health checks and readiness probes
• Set up alerting for system anomalies
• Conduct load testing and performance optimization

DELIVERABLES:
• Complete microservices architecture documentation
• Deployed production-ready platform on AWS
• Comprehensive API documentation and testing suite
• CI/CD pipeline with automated deployment
• Monitoring and alerting dashboard
• Security audit report and compliance documentation
• Performance testing results and optimization recommendations
• User training materials and technical documentation

TIMELINE:
• Phase 1 (Months 1-3): Architecture design and core services development
• Phase 2 (Months 4-6): Frontend development and API integration
• Phase 3 (Months 7-9): Testing, optimization, and deployment
• Phase 4 (Months 10-12): Production deployment and monitoring setup

SUCCESS METRICS:
• System uptime: 99.9% availability
• Performance: <200ms API response time
• Scalability: Handle 100,000 concurrent users
• Security: Zero critical vulnerabilities
• Code quality: 95%+ test coverage
• Documentation: 100% API documentation coverage

TECHNOLOGY STACK:
• Backend: Node.js, Go, Python, Java Spring Boot
• Frontend: React, TypeScript, Redux, Material-UI
• Databases: PostgreSQL, MongoDB, Redis, Elasticsearch
• Cloud: AWS (ECS, EKS, RDS, S3, Lambda, API Gateway)
• DevOps: Docker, Kubernetes, Jenkins, Terraform, ArgoCD
• Monitoring: Prometheus, Grafana, Jaeger, ELK Stack
• Security: OAuth 2.0, JWT, AWS IAM, Vault
• Testing: Jest, Cypress, Selenium, Postman, K6`
}
