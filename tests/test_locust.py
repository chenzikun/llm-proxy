import os
from locust import HttpUser, TaskSet, task, between

class OpenAITaskSet(TaskSet):

    @task
    def chat_completion(self):
        api_key = "sk-"  # 替换为你的 API 密钥

        # 请求头
        headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        }

        # 请求体
        data = {
            "model": "gpt-4o-mini",
            "messages": [
                {"role": "system", "content": "You are a helpful assistant."},
                {"role": "user", "content": "Tell me a joke."}
            ],
            "stream": False  # 设置为True时，locust不能直接接收流式返回
        }

        # 发起 POST 请求并捕获响应
        with self.client.post("/v1/chat/completions", json=data, headers=headers, catch_response=True) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"Failed with status code: {response.status_code}")


class OpenAIUser(HttpUser):
    tasks = [OpenAITaskSet]
    wait_time = between(1, 2)  # 模拟用户在 1 到 2 秒之间的等待时间
    host = "http://10.240.3.251:3500/v1"  # 指定目标地址


if __name__ == "__main__":
    import os
    os.system("locust -f locustfile.py")