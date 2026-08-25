from test_base import Base


class TestAzureProxy(Base):

    api_key = "sk-"
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {api_key}"
    }

    def test_stt(self):
        """Speech-To-Text 语音转文本"""
        data = self._http_post_formdata(
            url=self.STT_URL,
            data={"model": "transcible"},
            files={"file": open("audio.mp3", "rb")})
        print(data)

    def test_tts(self):
        """Text-To-Speech 文本转语音"""
        params = {
            "model": "polly",
            "input": "The quick brown fox jumped over the lazy dog.",
            "voice": "alloy"
        }
        r = self._http_post(self.TTS_URL, params=params)
        with open("audio.mp3", "wb+") as f:
            f.write(r.content)