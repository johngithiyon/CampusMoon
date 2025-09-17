package handlers

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

// Question represents an exam question
type Question struct {
	ID            int      `json:"id"`
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectAnswer int      `json:"correctAnswer"`
	Explanation   string   `json:"explanation"`
}

// ExamRequest represents a request for exam questions
type ExamRequest struct {
	Topic    string `json:"topic"`
	VideoID  string `json:"videoId"`
	Count    int    `json:"count"`
}

// ExamResponse represents a response with exam questions
type ExamResponse struct {
	Questions []Question `json:"questions"`
	Topic     string     `json:"topic"`
	VideoID   string     `json:"videoId"`
}

// AI questions
var aiQuestions = []Question{
	{
		ID:            1,
		Question:      "What does AI stand for?",
		Options:       []string{"Artificial Intelligence", "Automated Inference", "Algorithmic Integration", "Advanced Interface"},
		CorrectAnswer: 0,
		Explanation:   "AI stands for Artificial Intelligence, which refers to the simulation of human intelligence in machines.",
	},
	{
		ID:            2,
		Question:      "Which of these is a common application of AI?",
		Options:       []string{"Natural Language Processing", "Solar Panel Efficiency", "Hydropower Generation", "Wind Turbine Design"},
		CorrectAnswer: 0,
		Explanation:   "Natural Language Processing (NLP) is a common AI application that enables machines to understand and interpret human language.",
	},
	{
		ID:            3,
		Question:      "What is machine learning?",
		Options:       []string{"A subset of AI focused on algorithms that learn from data", "The process of building physical robots", "A type of renewable energy technology", "A data storage methodology"},
		CorrectAnswer: 0,
		Explanation:   "Machine learning is a subset of AI that focuses on developing algorithms that can learn from and make predictions based on data.",
	},
	{
		ID:            4,
		Question:      "Which AI technique is inspired by the human brain?",
		Options:       []string{"Neural Networks", "Decision Trees", "Support Vector Machines", "Random Forests"},
		CorrectAnswer: 0,
		Explanation:   "Neural networks are computing systems inspired by the biological neural networks in human brains.",
	},
	{
		ID:            5,
		Question:      "What is the Turing Test used for?",
		Options:       []string{"To evaluate a machine's ability to exhibit intelligent behavior", "To measure computing processing speed", "To assess data storage capacity", "To evaluate network security"},
		CorrectAnswer: 0,
		Explanation:   "The Turing Test evaluates a machine's ability to exhibit intelligent behavior equivalent to, or indistinguishable from, that of a human.",
	},
}

// VLSC questions
var vlscQuestions = []Question{
	{
		ID:            1,
		Question:      "What does VLSC stand for?",
		Options:       []string{"Very Large Scale Integration", "Variable Length Source Code", "Voltage Level System Control", "Visual Logic Simulation Circuit"},
		CorrectAnswer: 0,
		Explanation:   "VLSC stands for Very Large Scale Integration, which refers to the process of creating integrated circuits by combining thousands of transistors into a single chip.",
	},
	{
		ID:            2,
		Question:      "What is the primary goal of VLSC technology?",
		Options:       []string{"To increase transistor density on integrated circuits", "To reduce software development time", "To improve renewable energy efficiency", "To enhance AI algorithms"},
		CorrectAnswer: 0,
		Explanation:   "The primary goal of VLSC technology is to increase transistor density on integrated circuits, enabling more powerful and efficient electronic devices.",
	},
	{
		ID:            3,
		Question:      "Which industry benefits most directly from VLSC advancements?",
		Options:       []string{"Semiconductor industry", "Agriculture industry", "Textile industry", "Healthcare industry"},
		CorrectAnswer: 0,
		Explanation:   "The semiconductor industry benefits most directly from VLSC advancements as it deals with the design and manufacturing of integrated circuits.",
	},
	{
		ID:            4,
		Question:      "What is a common challenge in VLSC design?",
		Options:       []string{"Power dissipation and heat management", "Data storage limitations", "Software compatibility issues", "Network bandwidth constraints"},
		CorrectAnswer: 0,
		Explanation:   "Power dissipation and heat management are significant challenges in VLSC design due to the high density of transistors generating substantial heat.",
	},
	{
		ID:            5,
		Question:      "Which technology is closely related to VLSC?",
		Options:       []string{"CMOS technology", "Solar panel technology", "Wind turbine technology", "Hydroelectric technology"},
		CorrectAnswer: 0,
		Explanation:   "CMOS (Complementary Metal-Oxide-Semiconductor) technology is closely related to VLSC as it's a common technology used for constructing integrated circuits.",
	},
}

// Renewable Energy questions
var renewableQuestions = []Question{
	{
		ID:            1,
		Question:      "Which of these is a renewable energy source?",
		Options:       []string{"Solar power", "Natural gas", "Coal", "Petroleum"},
		CorrectAnswer: 0,
		Explanation:   "Solar power is a renewable energy source as it harnesses energy from the sun, which is virtually inexhaustible.",
	},
	{
		ID:            2,
		Question:      "What is the main advantage of renewable energy?",
		Options:       []string{"Reduced environmental impact", "Lower initial setup cost", "Consistent energy output regardless of weather", "Simpler technology"},
		CorrectAnswer: 0,
		Explanation:   "The main advantage of renewable energy is its reduced environmental impact compared to fossil fuels.",
	},
	{
		ID:            3,
		Question:      "Which renewable energy source uses photovoltaic cells?",
		Options:       []string{"Solar power", "Wind power", "Hydropower", "Geothermal energy"},
		CorrectAnswer: 0,
		Explanation:   "Solar power uses photovoltaic cells to convert sunlight directly into electricity.",
	},
	{
		ID:            4,
		Question:      "What is a common challenge for renewable energy?",
		Options:       []string{"Intermittency and storage issues", "High fuel costs", "Limited availability of resources", "Complex regulatory requirements"},
		CorrectAnswer: 0,
		Explanation:   "Intermittency and storage issues are common challenges for renewable energy, as sources like solar and wind are not always available.",
	},
	{
		ID:            5,
		Question:      "How can AI help renewable energy systems?",
		Options:       []string{"Optimizing energy production and distribution", "Reducing manufacturing costs of solar panels", "Increasing wind speed for turbines", "Creating more efficient hydroelectric dams"},
		CorrectAnswer: 0,
		Explanation:   "AI can help optimize energy production and distribution in renewable energy systems by predicting demand and managing grid operations.",
	},
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func GenerateExamHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var examReq ExamRequest
	err := json.NewDecoder(r.Body).Decode(&examReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	var questions []Question
	
	switch examReq.Topic {
	case "ai":
		questions = aiQuestions
	case "vlsc":
		questions = vlscQuestions
	case "renewable":
		questions = renewableQuestions
	default:
		questions = aiQuestions
	}
	
	// Shuffle questions
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(questions), func(i, j int) {
		questions[i], questions[j] = questions[j], questions[i]
	})
	
	// Limit to requested count
	if examReq.Count > 0 && examReq.Count < len(questions) {
		questions = questions[:examReq.Count]
	}
	
	response := ExamResponse{
		Questions: questions,
		Topic:     examReq.Topic,
		VideoID:   examReq.VideoID,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func ServeExam(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/exam.html")
}

