# ------------------- Full All-Languages EduBot (with STT + TTS) -------------------

import google.generativeai as genai
import traceback
from flask import Flask, request, jsonify, render_template, send_file
import speech_recognition as sr
from gtts import gTTS
import os
import uuid

# ------------------ CONFIG ------------------
genai.configure(api_key="AIzaSyAju-AEr0b_Hs2nmOh3NutBR7odmnVF4-4")  # replace with your API key
text_model = genai.GenerativeModel("gemini-1.5-flash")

# --- Fix paths so you can run from chat_server/ ---
BASE_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
TEMPLATES_DIR = os.path.join(BASE_DIR, "templates")
STATIC_DIR = os.path.join(BASE_DIR, "static")

app = Flask(__name__, template_folder=TEMPLATES_DIR, static_folder=STATIC_DIR)

UPLOAD_FOLDER = os.path.join(BASE_DIR, "uploads")
TTS_FOLDER = os.path.join(BASE_DIR, "tts")
os.makedirs(UPLOAD_FOLDER, exist_ok=True)
os.makedirs(TTS_FOLDER, exist_ok=True)

# ------------------ Languages ------------------
languages = [
    "English", "Hindi", "Tamil", "Rajasthani"
]

# ------------------ Greetings ------------------
greetings = {
    "English": "Hello!. Ask me anything about your chosen subject.",
    "Hindi": " नमस्ते! अपने चुने हुए विषय से संबंधित कोई भी प्रश्न पूछें।",
    "Tamil": " வணக்கம்! நீங்கள் தேர்ந்தெடுத்த பாடத்தைப் பற்றி கேளுங்கள்.",
    "Rajasthani": " राम राम! थारा चुना विषयां बारे पुछो."
}

# ------------------ Specialized subjects ---------------
specialized_subjects = {
    "English": ["VLSI", "Artificial Intelligence", "Renewable Energy"],
    "Hindi": ["बहुत बड़े पैमाने पर एकीकरण", "कृत्रिम बुद्धिमत्ता", "नवीकरणीय ऊर्जा"],
    "Tamil": ["மிகப் பெரிய அளவிலான ஒருங்கிணைப்பு", "கைநுட்ப நுண்ணறிவு", "புதுப்பிக்கத்தக்க சக்தி"],
    "Rajasthani": ["बड़ो पैमाणे पर एकीकरण", "कृत्रिम बुद्धि", "नवीकरणीय उर्जा"]
}

# ------------------ Exit Words, Farewells ------------------
messages = {
    "English": {
        "ready": "Specialized EduBot ready! Language = {lang}, Subject = {subj}",
        "instructions": "Type your questions about {subj} or type 'upload image' to analyze diagrams/notes.\nType 'exit' to quit."
    },
    "Hindi": {
        "ready": "विशेषीकृत EduBot तैयार! भाषा = {lang}, विषय = {subj}",
        "instructions": "{subj} संबंधित प्रश्न टाइप करें या डायग्राम/नोट्स का विश्लेषण करने के लिए 'upload image' टाइप करें।\nबाहर निकलने के लिए 'exit' टाइप करें।"
    },
    "Tamil": {
        "ready": "திறமையான EduBot தயாராக உள்ளது! மொழி = {lang}, பாடம் = {subj}",
        "instructions": "{subj} பற்றிய கேள்விகளை உள்ளிடவும் அல்லது வரைபடங்கள்/குறிப்புகளை பகுப்பாய்வு செய்ய 'upload image' எனத் தட்டவும்.\nமுடிக்க 'exit' எனத் தட்டவும்."
    },
    "Rajastani": {
        "ready": "विशेष EduBot तैयार! भाषा = {lang}, विषय = {subj}",
        "instructions": "{subj} पर सवाल लिखो, या 'upload image' लिख के नोट/डायाग्राम देखो।\nबाहर निकळबा खातर 'exit' लिखो।"
    }
}

exit_words = {
    "English": ["exit", "bye", "quit"],
    "Hindi": ["exit", "बाय", "निकलो", "बाहर"],
    "Tamil": ["exit", "வெளியேறு", "விடைபெறுகிறேன்"],
    "Rajasthani": ["exit", "छोड़ो", "राम राम", "बाय"]
}

# ------------------ Farewells ------------------
farewells = {
    "English": "Happy learning! Goodbye! ",
    "Hindi": "अच्छी पढ़ाई करो! अलविदा! ",
    "Tamil": "சிறந்த கற்றல் வாழ்த்துகள்! வணக்கம்! ",
    "Rajasthani": "खुशी सै पढ़ो! राम राम! "
}

# ------------------ TTS Language Map ------------------
tts_lang_map = {
    "English": "en", "Hindi": "hi", "Tamil": "ta", "Rajasthani": "hi"
}

# ------------------ Helper Functions ------------------
def get_response_text(resp):
    try:
        if resp is None:
            return "⚠️ No response."
        if hasattr(resp, "text") and resp.text:
            return resp.text
        if hasattr(resp, "candidates") and resp.candidates:
            cand = resp.candidates[0]
            if hasattr(cand, "content") and hasattr(cand.content, "parts"):
                part = cand.content.parts[0]
                if hasattr(part, "text"):
                    return part.text
            if hasattr(cand, "text"):
                return cand.text
            return str(cand)
        return str(resp)
    except Exception as e:
        return f"⚠️ Error extracting response: {e}"

def edu_bot_response(user_input, lang, subject):
    prompt = f"""
You are a top-tier expert and teacher in {subject}.
Answer the user's question with detailed explanations in {lang}.
Use headings, steps, bullet points, and examples.
User question: {user_input}
Answer:
"""
    try:
        resp = text_model.generate_content(prompt)
        return get_response_text(resp)
    except Exception as e:
        traceback.print_exc()
        return f"⚠️ Model call failed: {e}"

# ------------------ Flask Routes ------------------
@app.route("/")
def home():
    return render_template("branch_bot.html", greetings=greetings, subjects={"English": ["VLSI","AI","Renewable Energy"]})

@app.route("/ask", methods=["POST"])
def ask():
    data = request.json
    user_input = data.get("user_input", "")
    lang = data.get("language", "English")
    subject = data.get("subject", "General")
    reply = edu_bot_response(user_input, lang, subject)

    # Generate TTS file
    filename = f"{uuid.uuid4()}.mp3"
    filepath = os.path.join(TTS_FOLDER, filename)
    try:
        tts_lang = tts_lang_map.get(lang, "en")
        tts = gTTS(text=reply, lang=tts_lang, slow=False)
        tts.save(filepath)
        audio_url = f"/tts/{filename}"
    except Exception:
        audio_url = None

    return jsonify({"reply": reply, "audio_url": audio_url})

@app.route("/tts/<filename>")
def tts_file(filename):
    filepath = os.path.join(TTS_FOLDER, filename)
    return send_file(filepath, mimetype="audio/mpeg")

@app.route("/stt", methods=["POST"])
def stt():
    if "audio" not in request.files:
        return jsonify({"error": "No audio uploaded"}), 400
    file = request.files["audio"]
    filepath = os.path.join(UPLOAD_FOLDER, f"{uuid.uuid4()}.wav")
    file.save(filepath)

    recognizer = sr.Recognizer()
    with sr.AudioFile(filepath) as source:
        audio_data = recognizer.record(source)
        try:
            text = recognizer.recognize_google(audio_data)
        except sr.UnknownValueError:
            text = "⚠️ Could not understand audio."
        except sr.RequestError as e:
            text = f"⚠️ API error: {e}"
    os.remove(filepath)
    return jsonify({"text": text})

@app.route("/languages")
def get_languages():
    language_data = {}
    for lang in languages:
        language_data[lang] = {
            "greeting": greetings.get(lang, "Hello! I am EduBot."),
            "subjects": specialized_subjects.get(lang, ["General"])
        }
    return jsonify(language_data)

# ------------------ Run Server ------------------
if __name__ == "__main__":
    print("📂 Template folder:", app.template_folder)
    print("📂 Static folder:", app.static_folder)
    app.run(port=5000, debug=True)
