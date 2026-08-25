import requests


class Base(object):
    base_url = "http://localhost:3000/v1"
    api_key = None
    headers = None

    CHAT_URL = "/chat/completions"
    IMAGE_URL = "/images/generations"
    RERANK_URL = ""
    STT_URL = "/audio/transcriptions"
    TTS_URL = "/audio/speech"
    EMBEDDING_URL = "/embeddings"
    COMPLETION_URL = "/completions"
    MODERATION_URL = "/moderations"

    # 文件
    FILE_UPLOAD = "/files"
    FILE_LIST = "/files"
    FILE_RETRIEVE = "/files/{file_id}"
    FILE_DELETE = "/files/{file_id}"
    FILE_RETRIEVE_CONTENT = "/files/{file_id}/content"

    # 模型微调
    FINE_TUNING = "/fine_tuning/jobs"
    FINE_TUNING_LIST = "/fine_tuning/jobs"
    FINE_TUNING_EVENTS_LIST = "/fine_tuning/jobs/{job_id}/events"
    FINE_TUNING_RETRIEVE = "/fine_tuning/jobs/{job_id}"
    FINE_TUNING_CANCEL = "/fine_tuning/jobs/{job_id}/cancel"


    @classmethod
    def _build_url(cls, url):
        r = cls.base_url + url
        # print(r)
        return r
    
    @classmethod
    def _http_get(cls, url, params=None):
        r = requests.get(cls._build_url(url),
                          params=params,
                          headers=cls.headers)
        if not r.ok:
            raise requests.HTTPError(
                f"status code={r.status_code}, response={r.text}",
                request=r.request,
                response=r)
        if "application/json" in r.headers["Content-Type"]:
            return r.json()
        return r

    @classmethod
    def _http_post(cls, url, params=None):
        r = requests.post(cls._build_url(url),
                          json=params,
                          headers=cls.headers)
        if not r.ok:
            raise requests.HTTPError(
                f"status code={r.status_code}, response={r.text}",
                request=r.request,
                response=r)
        if "application/json" in r.headers["Content-Type"]:
            return r.json()
        return r

    @classmethod
    def _http_post_formdata(cls, url, data=None, files=None):
        r = requests.post(
            cls._build_url(url),
            data=data,
            files=files,
            headers={"Authorization": cls.headers["Authorization"]})
        if not r.ok:
            raise requests.HTTPError(
                f"status code={r.status_code}, response={r.text}",
                request=r.request,
                response=r)
        if r.headers["Content-Type"] == "application/json":
            return r.json()
        return r
    
    @classmethod
    def _http_delete(cls, url):
        r = requests.delete(cls._build_url(url),
                          headers=cls.headers)
        if not r.ok:
            raise requests.HTTPError(
                f"status code={r.status_code}, response={r.text}",
                request=r.request,
                response=r)
        if "application/json" in r.headers["Content-Type"]:
            return r.json()
        return r
    
    def test_chat(self):
        pass

    def test_embedding(self):
        pass

    def test_image(self):
        pass

    def test_rerank(self):
        pass

    def test_stt(self):
        pass

    def test_tts(self):
        pass