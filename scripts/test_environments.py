"""
环境连接测试脚本
在实际同步前测试所有环境的连接性和认证

使用方法:
    python test_environments.py
"""

import requests
import logging
from typing import Dict
import urllib3
from datetime import datetime
import concurrent.futures

# 禁用SSL警告
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# 认证信息
USERNAME = "root"
PASSWORD = "proxy@autel"

# 环境配置
ENVIRONMENTS = {
    "eneprodus": {
        "name": "美国生产",
        "url": "https://ai-llms-proxy-eneprodus.autel.com",
        "is_source": True
    },
    "dev": {
        "name": "中国开发",
        "url": "https://ai-llms-proxy.auteltech.cn",
        "is_source": False
    },
    "enetest": {
        "name": "中国测试",
        "url": "https://ai-llms-proxy-enetest.auteltech.cn",
        "is_source": False
    },
    "eneprod": {
        "name": "中国生产",
        "url": "https://ai-llms-proxy-eneprod.auteltech.cn",
        "is_source": False
    },
    "enetestus": {
        "name": "美西测试",
        "url": "https://ai-llms-proxy-enetestus.autel.com",
        "is_source": False
    },
    "eneprodca": {
        "name": "加拿大OCI",
        "url": "https://ai-llms-proxy-eneprodca.autel.com",
        "is_source": False
    },
    "eneprodeu": {
        "name": "欧洲生产",
        "url": "https://ai-llms-proxy-eneprodeu.autel.com",
        "is_source": False
    },
    "eneprodap": {
        "name": "新能源亚太",
        "url": "https://ai-llms-proxy-eneprodap.autel.com",
        "is_source": False
    }
}


def test_environment(env_key: str, env_config: Dict) -> Dict:
    """测试单个环境的连接性"""
    result = {
        "env_key": env_key,
        "env_name": env_config["name"],
        "url": env_config["url"],
        "is_source": env_config.get("is_source", False),
        "network_ok": False,
        "login_ok": False,
        "api_ok": False,
        "model_count": 0,
        "error": None,
        "response_time": None
    }
    
    session = requests.Session()
    session.verify = False
    
    try:
        # 1. 测试网络连接
        start_time = datetime.now()
        status_url = f"{env_config['url']}/api/status"
        response = session.get(status_url, timeout=5)
        response_time = (datetime.now() - start_time).total_seconds()
        result["response_time"] = response_time
        
        if response.status_code == 200:
            result["network_ok"] = True
        else:
            result["error"] = f"HTTP {response.status_code}"
            return result
        
        # 2. 测试登录
        login_url = f"{env_config['url']}/api/user/login"
        data = {
            "username": USERNAME,
            "password": PASSWORD
        }
        response = session.post(login_url, json=data, timeout=10)
        
        if response.status_code == 200:
            res_data = response.json()
            if res_data.get("success"):
                result["login_ok"] = True
            else:
                result["error"] = f"登录失败: {res_data.get('message')}"
                return result
        else:
            result["error"] = f"登录失败: HTTP {response.status_code}"
            return result
        
        # 3. 测试API访问（获取模型列表）
        api_url = f"{env_config['url']}/api/model-meta/"
        params = {"p": 0, "pageSize": 10}
        response = session.get(api_url, params=params, timeout=10)
        
        if response.status_code == 200:
            res_data = response.json()
            if res_data.get("success"):
                result["api_ok"] = True
                # 获取总模型数（需要再请求一次大数量）
                params_all = {"p": 0, "pageSize": 10000}
                response_all = session.get(api_url, params=params_all, timeout=10)
                if response_all.status_code == 200:
                    res_all = response_all.json()
                    if res_all.get("success"):
                        result["model_count"] = len(res_all.get("data", []))
            else:
                result["error"] = f"API失败: {res_data.get('message')}"
                return result
        else:
            result["error"] = f"API失败: HTTP {response.status_code}"
            return result
        
    except requests.exceptions.Timeout:
        result["error"] = "连接超时"
    except requests.exceptions.ConnectionError:
        result["error"] = "无法连接"
    except Exception as e:
        result["error"] = f"异常: {str(e)}"
    
    return result


def test_all_environments():
    """测试所有环境"""
    logger.info("=" * 80)
    logger.info("开始测试所有环境的连接性")
    logger.info("=" * 80)
    
    results = []
    
    # 使用线程池并发测试（提高速度）
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
        future_to_env = {
            executor.submit(test_environment, key, config): (key, config)
            for key, config in ENVIRONMENTS.items()
        }
        
        for future in concurrent.futures.as_completed(future_to_env):
            result = future.result()
            results.append(result)
    
    # 排序结果：源环境在前
    results.sort(key=lambda x: (not x["is_source"], x["env_name"]))
    
    # 打印结果
    logger.info(f"\n{'=' * 80}")
    logger.info("测试结果汇总")
    logger.info(f"{'=' * 80}\n")
    
    success_count = 0
    failed_count = 0
    
    for result in results:
        env_type = "源环境" if result["is_source"] else "目标环境"
        logger.info(f"{'-' * 80}")
        logger.info(f"环境: {result['env_name']} ({env_type})")
        logger.info(f"地址: {result['url']}")
        logger.info(f"环境Key: {result['env_key']}")
        
        if result["response_time"]:
            logger.info(f"响应时间: {result['response_time']:.2f}秒")
        
        # 测试结果
        status_line = []
        if result["network_ok"]:
            status_line.append("✓ 网络连接")
        else:
            status_line.append("✗ 网络连接")
        
        if result["login_ok"]:
            status_line.append("✓ 登录认证")
        else:
            status_line.append("✗ 登录认证")
        
        if result["api_ok"]:
            status_line.append("✓ API访问")
        else:
            status_line.append("✗ API访问")
        
        logger.info(f"状态: {' | '.join(status_line)}")
        
        if result["api_ok"]:
            logger.info(f"模型数量: {result['model_count']}")
            success_count += 1
        
        if result["error"]:
            logger.info(f"错误信息: {result['error']}")
            failed_count += 1
        
        # 总体评估
        if result["network_ok"] and result["login_ok"] and result["api_ok"]:
            logger.info("✓ 环境可用，可以进行同步")
        else:
            logger.info("✗ 环境不可用，需要排查问题")
    
    # 打印统计
    logger.info(f"\n{'=' * 80}")
    logger.info("测试统计")
    logger.info(f"{'=' * 80}")
    logger.info(f"总环境数: {len(results)}")
    logger.info(f"测试通过: {success_count} ✓")
    logger.info(f"测试失败: {failed_count} ✗")
    
    if failed_count == 0:
        logger.info(f"\n✓ 所有环境测试通过，可以运行同步脚本")
        logger.info(f"\n建议:")
        logger.info(f"  1. 先运行干运行模式查看差异:")
        logger.info(f"     python sync_model_pricing_advanced.py --dry-run")
        logger.info(f"  2. 导出差异报告:")
        logger.info(f"     python sync_model_pricing_advanced.py --dry-run --report diff.csv")
        logger.info(f"  3. 确认无误后执行实际同步:")
        logger.info(f"     python sync_model_pricing.py")
    else:
        logger.info(f"\n✗ 部分环境测试失败，请先解决连接问题")
        logger.info(f"\n常见问题:")
        logger.info(f"  - 网络连接: 检查网络是否可达，防火墙设置")
        logger.info(f"  - 登录失败: 检查用户名密码是否正确")
        logger.info(f"  - API失败: 检查账号是否有管理员权限")
    
    logger.info(f"{'=' * 80}\n")
    
    return success_count == len(results)


if __name__ == "__main__":
    try:
        test_all_environments()
    except KeyboardInterrupt:
        logger.info("\n用户中断操作")
    except Exception as e:
        logger.error(f"\n发生未预期的错误: {str(e)}", exc_info=True)

