from flask import Flask, request, jsonify, render_template
from difflib import get_close_matches
import os

# Set up Flask with custom template directory
BASE_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
TEMPLATES_DIR = os.path.join(BASE_DIR, "templates")
app = Flask(__name__, template_folder=TEMPLATES_DIR)

# AI Q&A dictionary
ai_answers = {
    "tell about the video": "This Video is about Artificial Intelligence",
    "what is ai": "Artificial Intelligence (AI) refers to the simulation of human intelligence in machines that are programmed to think like humans and mimic their actions.",
    "what is machine learning": "Machine Learning (ML) is a subset of AI where machines learn patterns from data and improve performance without explicit programming.",
    "what is deep learning": "Deep Learning uses multi-layered neural networks to automatically learn features and representations from data, often used in image and speech recognition.",
    "what is neural network": "A Neural Network is a computational model inspired by the human brain, designed to recognize patterns and solve complex tasks.",
    "what is supervised learning": "Supervised Learning is a type of ML where models are trained on labeled datasets to make predictions on unseen data.",
    "what is unsupervised learning": "Unsupervised Learning finds hidden patterns in data without labeled responses, commonly used for clustering and association tasks.",
    "what is reinforcement learning": "Reinforcement Learning is an AI technique where agents learn by interacting with the environment and receiving rewards or penalties.",
    "what is natural language processing": "NLP is a field of AI that enables computers to understand, interpret, and generate human language.",
    "what is computer vision": "Computer Vision allows machines to interpret visual information from the world, such as images and videos, for decision-making.",
    "what is robotics": "Robotics combines AI, sensors, and actuators to create intelligent machines capable of performing tasks autonomously.",
    "what is chatgpt": "ChatGPT is an AI model developed by OpenAI that can generate human-like text, answer questions, and engage in conversations.",
    "what is generative ai": "Generative AI creates new content such as text, images, or music based on existing data patterns.",
    "how does ai work": "AI works by learning patterns from data, using algorithms and models to make predictions, decisions, or generate content."
}

# Add greetings and farewells
greetings = ["hi", "hii", "hello", "hey", "good morning", "good evening"]
farewells = ["bye", "goodbye", "see you", "later"]

@app.route("/videobot")
def home():
    return render_template("video_bot.html")

@app.route("/ask", methods=["POST"])
def ask():
    data = request.get_json()
    question = data.get("question", "").lower().strip()

    # Respond to greetings and farewells
    if question in greetings:
        return jsonify({"answer": "Hello! How can I help you today?"})
    if question in farewells:
        return jsonify({"answer": "Goodbye! Have a great day!"})

    # Fuzzy matching for AI questions
    closest = get_close_matches(question, ai_answers.keys(), n=1, cutoff=0.5)
    if closest:
        answer = ai_answers[closest[0]]
    else:
        answer = "Sorry, I can only answer AI-related questions."

    return jsonify({"answer": answer})

if __name__ == "__main__":
    app.run(debug=True)
