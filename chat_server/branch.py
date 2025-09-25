from flask import Flask, render_template, request, jsonify, send_file
from gtts import gTTS
import os
import difflib

app = Flask(__name__)


# Set up Flask with custom template directory
BASE_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
TEMPLATES_DIR = os.path.join(BASE_DIR, "templates")
app = Flask(__name__, template_folder=TEMPLATES_DIR)

# ------------------- DATA -------------------

subjects = {
    "en": {
        "AI": {
            "What is AI?": "AI is the simulation of human intelligence in machines.",
            "What is Machine Learning?": "Machine Learning is a subset of AI that learns from data.",
            "What is NLP?": "NLP is Natural Language Processing, enabling machines to understand human language.",
            "What is Computer Vision?": "Computer Vision allows machines to interpret and understand images.",
            "What is Robotics?": "Robotics combines AI with mechanical systems to perform tasks."
        },
        "VLSI": {
            "What is VLSI?": "VLSI means Very Large Scale Integration, where thousands of transistors are integrated into a chip.",
            "What is CMOS?": "CMOS is Complementary Metal-Oxide-Semiconductor technology.",
            "What is FPGA?": "FPGA is Field Programmable Gate Array, a reconfigurable IC.",
            "What is ASIC?": "ASIC is Application Specific Integrated Circuit, designed for a specific task.",
            "What is SoC?": "SoC is System on Chip, integrating all components on a single chip.",
            "Tell me about VLSI": "VLSI stands for Very Large Scale Integration, a process of creating integrated circuits by combining thousands of transistors into a single chip."
        },
        "Renewable Energy": {
            "What is Solar Energy?": "Solar energy is energy from the Sun converted into electricity.",
            "What is Wind Energy?": "Wind energy is generated using wind turbines.",
            "What is Biomass Energy?": "Biomass energy comes from organic matter like plants and waste.",
            "What is Hydropower?": "Hydropower uses flowing water to generate electricity.",
            "What is Geothermal Energy?": "Geothermal energy comes from heat inside the Earth."
        }
    },
    "hi": {
        "AI": {
            "एआई क्या है?": "एआई मशीनों में मानव बुद्धि का अनुकरण है।",
            "मशीन लर्निंग क्या है?": "मशीन लर्निंग एआई का एक हिस्सा है जो डाटा से सीखता है।",
            "एनएलपी क्या है?": "एनएलपी प्राकृतिक भाषा प्रसंस्करण है।",
            "कंप्यूटर विज़न क्या है?": "कंप्यूटर विज़न मशीनों को चित्र समझने में सक्षम बनाता है।",
            "रोबोटिक्स क्या है?": "रोबोटिक्स एआई और यांत्रिक प्रणाली को जोड़कर कार्य करता है।"
        },
        "VLSI": {
            "वीएलएसआई क्या है?": "वीएलएसआई का अर्थ है वेरी लार्ज स्केल इंटीग्रेशन।",
            "सीएमओएस क्या है?": "सीएमओएस का अर्थ है कॉम्प्लीमेंटरी मेटल ऑक्साइड सेमीकंडक्टर।",
            "एफपीजीए क्या है?": "एफपीजीए एक प्रोग्रामेबल आईसी है।",
            "एएसआईसी क्या है?": "एएसआईसी का अर्थ है एप्लिकेशन स्पेसिफिक इंटीग्रेटेड सर्किट।",
            "एसओसी क्या है?": "एसओसी का अर्थ है सिस्टम ऑन चिप।"
        },
        "Renewable Energy": {
            "सौर ऊर्जा क्या है?": "सौर ऊर्जा सूर्य से प्राप्त ऊर्जा है।",
            "पवन ऊर्जा क्या है?": "पवन ऊर्जा पवन टर्बाइन से उत्पन्न होती है।",
            "बायोमास ऊर्जा क्या है?": "बायोमास ऊर्जा पौधों और अपशिष्ट से आती है।",
            "जलविद्युत क्या है?": "जलविद्युत बहते पानी से उत्पन्न होती है।",
            "भू-तापीय ऊर्जा क्या है?": "भू-तापीय ऊर्जा पृथ्वी के अंदर की गर्मी से आती है।"
        }
    },
    "ta": {
        "AI": {
            "செயற்கை நுண்ணறிவு என்றால் என்ன": "AI என்பது இயந்திரங்களில் மனித நுண்ணறிவைப் பின்பற்றுவது.",
            "இயந்திர கற்றல் என்றால் என்ன?": "Machine Learning என்பது தரவிலிருந்து கற்றுக்கொள்வது.",
            "NLP என்றால் என்ன?": "NLP என்பது இயற்கை மொழி செயலாக்கம்.",
            "கணினி பார்வை என்றால் என்ன?": "Computer Vision என்பது படங்களை புரிந்து கொள்ளும் திறன்.",
            "ரோபோடிக் என்றால் என்ன?": "Robotics என்பது AI மற்றும் இயந்திர அமைப்புகளின் இணைப்பு."
        },
        "VLSI": {
            "VLSI என்றால் என்ன?": "VLSI என்பது Very Large Scale Integration.",
            "CMOS என்றால் என்ன?": "CMOS என்பது Complementary Metal-Oxide-Semiconductor தொழில்நுட்பம்.",
            "FPGA என்றால் என்ன?": "FPGA என்பது Field Programmable Gate Array.",
            "ASIC என்றால் என்ன?": "ASIC என்பது Application Specific Integrated Circuit.",
            "SoC என்றால் என்ன?": "SoC என்பது System on Chip."
        },
        "Renewable Energy": {
            "சூரிய ஆற்றல் என்றால் என்ன?": "சூரிய ஆற்றல் என்பது சூரியனிடமிருந்து பெறப்படும் ஆற்றல்.",
            "காற்றாலை ஆற்றல் என்றால் என்ன?": "காற்றாலை ஆற்றல் காற்றாலைகளால் உற்பத்தி செய்யப்படுகிறது.",
            "பயோமாஸ் ஆற்றல் என்றால் என்ன?": "பயோமாஸ் ஆற்றல் தாவரங்கள் மற்றும் கழிவுகளிலிருந்து பெறப்படுகிறது.",
            "நீர்வழி மின்சாரம் என்றால் என்ன?": "நீர்வழி மின்சாரம் ஓடும் நீரிலிருந்து பெறப்படுகிறது.",
            "பூமியாழ் ஆற்றல் என்றால் என்ன?": "பூமியாழ் ஆற்றல் பூமியின் உள் சூட்டிலிருந்து பெறப்படுகிறது."
        }
    },
    "rj": {
        "AI": {
            "एआई काइ है?": "एआई मशीनां में मानव बुद्धि को नकल करै है।",
            "मशीन लर्निंग काइ है?": "मशीन लर्निंग डाटा सिखै है।",
            "एनएलपी काइ है?": "एनएलपी मशीनां ने भाषा समझणो सिखावै है।",
            "कंप्यूटर विजन काइ है?": "कंप्यूटर विजन चित्र समझणो सिखावै है।",
            "रोबोटिक्स काइ है?": "रोबोटिक्स मशीनां अौर एआई ने जोड़ै है।"
        },
        "VLSI": {
            "वीएलएसआई काइ है?": "वीएलएसआई मतलब बहुत वड्डा स्केल इंटीग्रेशन।",
            "सीएमओएस काइ है?": "सीएमओएस मतलब कॉम्प्लीमेंटरी मेटल ऑक्साइड।",
            "एफपीजीए काइ है?": "एफपीजीए एक प्रोग्रामेबल आईसी है।",
            "एएसआईसी काइ है?": "एएसआईसी मतलब एप्लिकेशन स्पेसिफिक आईसी।",
            "एसओसी काइ है?": "एसओसी मतलब सिस्टम ऑन चिप।"
        },
        "Renewable Energy": {
            "सोलर एनर्जी काइ है?": "सोलर एनर्जी सूरज सै मिलै है।",
            "हवा एनर्जी काइ है?": "हवा एनर्जी पंखा टर्बाइन सै बनै है।",
            "बायोमास एनर्जी काइ है?": "बायोमास एनर्जी पौधां अौर कचरा सै मिलै है।",
            "पाणी बत्ती काइ है?": "पाणी बत्ती बहता पाणी सै बनै है।",
            "जमीन गूर्म एनर्जी काइ है?": "जमीन गूर्म एनर्जी धरती कू गूर्म सै मिलै है।"
        }
    }
}

# Greetings + Farewells
greetings = {
    "en": "Hello! How can I help you today?",
    "hi": "नमस्ते! मैं आपकी कैसे मदद कर सकता हूँ?",
    "ta": "வணக்கம்! உங்களுக்கு எப்படி உதவலாம்?",
    "rj": "राम राम! म्हे थारी कद मदद करूं?"
}

farewells = {
    "en": "Goodbye! Have a great day!",
    "hi": "अलविदा! आपका दिन शुभ हो!",
    "ta": "பிரியாவிடை! நல்ல நாளாக இருக்கட்டும்!",
    "rj": "राम राम! थारो दिन मंगलमय होवे!"
}

# ------------------- STATE -------------------
current_lang = "en"

# ------------------- ROUTES -------------------

@app.route("/branchbot")
def index():
    return render_template("branch_bot.html")

@app.route("/ask", methods=["POST"])
def ask():
    global current_lang
    data = request.get_json()
    question = data.get("question", "").strip()
    subject = data.get("subject", "").strip()

    # Greetings
    if question.lower() in ["hi", "hello", "hey", "नमस्ते", "வணக்கம்", "राम राम"]:
        return jsonify({"answer": greetings[current_lang]})

    # Farewells
    if question.lower() in ["bye", "goodbye", "exit", "अलविदा", "பிரியாவிடை", "राम राम"]:
        return jsonify({"answer": farewells[current_lang]})

    # Subject Q&A with fuzzy matching
    answer = None
    lang_data = subjects.get(current_lang, {})
    
    # Try to find answer in the selected subject first
    if subject in lang_data:
        qa = lang_data[subject]
        possible_questions = list(qa.keys())
        match = difflib.get_close_matches(question, possible_questions, n=1, cutoff=0.6)
        if match:
            answer = qa[match[0]]
    
    # If not found in selected subject, search all subjects
    if not answer:
        for subj, qa in lang_data.items():
            possible_questions = list(qa.keys())
            match = difflib.get_close_matches(question, possible_questions, n=1, cutoff=0.6)
            if match:
                answer = qa[match[0]]
                break

    if not answer:
        answer = "Sorry, I don't know that one." if current_lang == "en" else greetings[current_lang]

    return jsonify({"answer": answer})

@app.route("/tts")
def tts():
    text = request.args.get("text", "")
    lang = request.args.get("lang", "en")
    tts = gTTS(text=text, lang=lang)
    filepath = "response.mp3"
    tts.save(filepath)
    return send_file(filepath, mimetype="audio/mpeg")

@app.route("/set_language", methods=["POST"])
def set_language():
    global current_lang
    data = request.get_json()
    lang = data.get("lang", "en")
    if lang in subjects:
        current_lang = lang
    return jsonify({"status": "ok", "lang": current_lang, "greeting": greetings[current_lang]})

# ------------------- MAIN -------------------
if __name__ == "__main__":
    app.run(debug=True)