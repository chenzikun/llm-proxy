#!/usr/bin/env python3
"""
模型价格同步脚本
从美国生产环境同步模型价格到其他环境

使用方法:
    python sync_model_pricing.py
"""

import requests
import json
import logging
from typing import Dict, List, Optional
import urllib3

# 禁用SSL警告（如果使用自签名证书）
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
    # 源环境（美国生产）
    "eneprodus": {
        "name": "美国生产",
        "url": "https://ai-llms-proxy-eneprodus.autel.com",
        "is_source": True
    },
    # 目标环境
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


class ModelPricingSync:
    def __init__(self, env_config: dict):
        self.env_name = env_config["name"]
        self.base_url = env_config["url"]
        self.session = requests.Session()
        self.session.verify = False  # 如果使用自签名证书
        self.token = None
    
    def login(self) -> bool:
        """登录并获取token"""
        try:
            login_url = f"{self.base_url}/api/user/login"
            data = {
                "username": USERNAME,
                "password": PASSWORD
            }
            
            logger.info(f"正在登录 {self.env_name} ({self.base_url})...")
            response = self.session.post(login_url, json=data, timeout=10)
            
            if response.status_code == 200:
                result = response.json()
                if result.get("success"):
                    # Token可能在cookie中，或者在响应中
                    self.token = result.get("data")
                    logger.info(f"✓ {self.env_name} 登录成功")
                    return True
                else:
                    logger.error(f"✗ {self.env_name} 登录失败: {result.get('message')}")
                    return False
            else:
                logger.error(f"✗ {self.env_name} 登录失败: HTTP {response.status_code}")
                return False
        except Exception as e:
            logger.error(f"✗ {self.env_name} 登录异常: {str(e)}")
            return False
    
    def get_all_models(self) -> Optional[List[Dict]]:
        """获取所有模型列表"""
        try:
            url = f"{self.base_url}/api/model-meta/"
            params = {"p": 0, "pageSize": 10000}  # 获取所有模型
            
            logger.info(f"正在获取 {self.env_name} 的模型列表...")
            response = self.session.get(url, params=params, timeout=10)
            
            if response.status_code == 200:
                result = response.json()
                if result.get("success"):
                    models = result.get("data", [])
                    logger.info(f"✓ {self.env_name} 获取到 {len(models)} 个模型")
                    return models
                else:
                    logger.error(f"✗ {self.env_name} 获取模型失败: {result.get('message')}")
                    return None
            else:
                logger.error(f"✗ {self.env_name} 获取模型失败: HTTP {response.status_code}")
                return None
        except Exception as e:
            logger.error(f"✗ {self.env_name} 获取模型异常: {str(e)}")
            return None
    
    def update_model_pricing(self, model_name: str, model_ratio: float, completion_ratio: float) -> bool:
        """更新单个模型的价格"""
        try:
            url = f"{self.base_url}/api/model-meta/update_ratio"
            data = {
                "model": model_name,
                "model_ratio": model_ratio,
                "completion_ratio": completion_ratio
            }
            
            response = self.session.post(url, json=data, timeout=10)
            
            if response.status_code == 200:
                result = response.json()
                if result.get("success"):
                    return True
                else:
                    logger.warning(f"  ✗ 更新模型 {model_name} 失败: {result.get('message')}")
                    return False
            else:
                logger.warning(f"  ✗ 更新模型 {model_name} 失败: HTTP {response.status_code}")
                return False
        except Exception as e:
            logger.warning(f"  ✗ 更新模型 {model_name} 异常: {str(e)}")
            return False


def sync_pricing():
    """主同步函数"""
    logger.info("=" * 80)
    logger.info("开始同步模型价格")
    logger.info("=" * 80)
    
    # 1. 获取源环境配置
    source_env_key = None
    for key, config in ENVIRONMENTS.items():
        if config.get("is_source"):
            source_env_key = key
            break
    
    if not source_env_key:
        logger.error("未找到源环境配置")
        return
    
    source_config = ENVIRONMENTS[source_env_key]
    
    # 2. 从源环境获取模型价格
    logger.info(f"\n{'=' * 80}")
    logger.info(f"步骤 1: 从源环境 {source_config['name']} 获取模型价格")
    logger.info(f"{'=' * 80}")
    
    source_sync = ModelPricingSync(source_config)
    if not source_sync.login():
        logger.error("源环境登录失败，终止同步")
        return
    
    source_models = source_sync.get_all_models()
    if not source_models:
        logger.error("无法获取源环境模型列表，终止同步")
        return
    
    # 构建模型价格映射表
    source_pricing = {}
    for model in source_models:
        model_name = model.get("model")
        model_ratio = model.get("model_ratio")
        completion_ratio = model.get("completion_ratio")
        
        if model_name and model_ratio is not None and completion_ratio is not None:
            source_pricing[model_name] = {
                "model_ratio": model_ratio,
                "completion_ratio": completion_ratio,
                "channel_type": model.get("channel_type")
            }
    
    logger.info(f"✓ 源环境共有 {len(source_pricing)} 个模型有价格信息")
    
    # 3. 同步到目标环境
    logger.info(f"\n{'=' * 80}")
    logger.info(f"步骤 2: 同步价格到目标环境")
    logger.info(f"{'=' * 80}\n")
    
    summary = {
        "total_envs": 0,
        "success_envs": 0,
        "failed_envs": 0,
        "total_models": 0,
        "updated_models": 0,
        "failed_models": 0,
        "skipped_models": 0
    }
    
    for env_key, env_config in ENVIRONMENTS.items():
        if env_config.get("is_source"):
            continue  # 跳过源环境
        
        summary["total_envs"] += 1
        
        logger.info(f"\n{'-' * 80}")
        logger.info(f"正在同步到: {env_config['name']}")
        logger.info(f"{'-' * 80}")
        
        # 登录目标环境
        target_sync = ModelPricingSync(env_config)
        if not target_sync.login():
            summary["failed_envs"] += 1
            logger.error(f"✗ {env_config['name']} 登录失败，跳过该环境\n")
            continue
        
        # 获取目标环境的模型列表
        target_models = target_sync.get_all_models()
        if not target_models:
            summary["failed_envs"] += 1
            logger.error(f"✗ {env_config['name']} 获取模型列表失败，跳过该环境\n")
            continue
        
        # 构建目标环境模型映射（按模型名）
        target_models_map = {model.get("model"): model for model in target_models}
        
        # 同步价格
        env_updated = 0
        env_failed = 0
        env_skipped = 0
        
        for model_name, pricing in source_pricing.items():
            if model_name not in target_models_map:
                # 目标环境没有这个模型，跳过
                env_skipped += 1
                continue
            
            # 检查价格是否已经一致
            target_model = target_models_map[model_name]
            current_model_ratio = target_model.get("model_ratio")
            current_completion_ratio = target_model.get("completion_ratio")
            
            if (current_model_ratio == pricing["model_ratio"] and 
                current_completion_ratio == pricing["completion_ratio"]):
                # 价格已经一致，跳过
                env_skipped += 1
                continue
            
            # 更新价格
            logger.info(f"  更新模型: {model_name}")
            logger.info(f"    输入价格: {current_model_ratio} -> {pricing['model_ratio']}")
            logger.info(f"    输出价格: {current_completion_ratio} -> {pricing['completion_ratio']}")
            
            if target_sync.update_model_pricing(
                model_name,
                pricing["model_ratio"],
                pricing["completion_ratio"]
            ):
                env_updated += 1
                logger.info(f"  ✓ 更新成功")
            else:
                env_failed += 1
        
        # 统计
        summary["success_envs"] += 1
        summary["total_models"] += len(target_models_map)
        summary["updated_models"] += env_updated
        summary["failed_models"] += env_failed
        summary["skipped_models"] += env_skipped
        
        logger.info(f"\n{env_config['name']} 同步完成:")
        logger.info(f"  - 目标环境模型数: {len(target_models_map)}")
        logger.info(f"  - 更新成功: {env_updated}")
        logger.info(f"  - 更新失败: {env_failed}")
        logger.info(f"  - 跳过（无此模型或价格相同）: {env_skipped}")
    
    # 4. 打印总结
    logger.info(f"\n{'=' * 80}")
    logger.info("同步完成 - 总结")
    logger.info(f"{'=' * 80}")
    logger.info(f"环境统计:")
    logger.info(f"  - 目标环境总数: {summary['total_envs']}")
    logger.info(f"  - 同步成功: {summary['success_envs']}")
    logger.info(f"  - 同步失败: {summary['failed_envs']}")
    logger.info(f"\n模型统计:")
    logger.info(f"  - 目标环境模型总数: {summary['total_models']}")
    logger.info(f"  - 价格更新成功: {summary['updated_models']}")
    logger.info(f"  - 价格更新失败: {summary['failed_models']}")
    logger.info(f"  - 跳过（无此模型或价格相同）: {summary['skipped_models']}")
    logger.info(f"{'=' * 80}\n")


if __name__ == "__main__":
    try:
        sync_pricing()
    except KeyboardInterrupt:
        logger.info("\n用户中断操作")
    except Exception as e:
        logger.error(f"\n发生未预期的错误: {str(e)}", exc_info=True)

