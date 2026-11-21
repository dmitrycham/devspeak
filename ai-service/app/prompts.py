ENRICH_PROMPT = """\
You help a Russian-speaking software developer learn English.
Explain the English phrase below for a vocabulary card.

Phrase: {phrase}

Answer with JSON that has exactly these string fields:
- translation: the translation into Russian
- example: one short English sentence that uses the phrase
- it_context: one short English sentence about how developers use it at work
"""
