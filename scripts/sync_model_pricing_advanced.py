#!/usr/bin/env python3
"""
模型价格同步脚本（增强版）
从美国生产环境同步模型价格到其他环境

增强功能：
- 支持干运行模式（--dry-run）
- 支持只同步指定环境（--env）
- 支持导出价格差异报告（--report）
- 支持只同步特定模型（--models）

使用方法:
    # 干运行模式（不实际更新，只查看差异）
    python sync_model_pricing_advanced.py --dry-run
    
    # 只同步到指定环境
    python sync_model_pricing_advanced.py --env dev,enetest
    
    # 导出差异报告到CSV
    python sync_model_pricing_advanced.py --report pricing_diff.csv
    
    # 只同步特定模型
    python sync_model_pricing_advanced.py --models gpt-4o,gpt-4o-mini
"""

import requests
import json
import logging
import argparse
import csv
from typing import Dict, List, Optional, Set
import urllib3
from datetime import datetime

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


class ModelPricingSync:
    def __init__(self, env_config: dict):
        self.env_name = env_config["name"]
        self.base_url = env_config["url"]
        self.session = requests.Session()
        self.session.verify = False
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
            params = {"p": 0, "pageSize": 10000}
            
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


def export_diff_report(differences: List[Dict], report_file: str):
    """导出价格差异报告到CSV"""
    try:
        with open(report_file, 'w', newline='', encoding='utf-8') as f:
            fieldnames = [
                '环境', '模型名称', 
                '源环境输入价格', '目标环境输入价格', '输入价格差异',
                '源环境输出价格', '目标环境输出价格', '输出价格差异',
                '状态'
            ]
            writer = csv.DictWriter(f, fieldnames=fieldnames)
            
            writer.writeheader()
            for diff in differences:
                writer.writerow({
                    '环境': diff['env_name'],
                    '模型名称': diff['model_name'],
                    '源环境输入价格': diff['source_model_ratio'],
                    '目标环境输入价格': diff['target_model_ratio'],
                    '输入价格差异': diff['model_ratio_diff'],
                    '源环境输出价格': diff['source_completion_ratio'],
                    '目标环境输出价格': diff['target_completion_ratio'],
                    '输出价格差异': diff['completion_ratio_diff'],
                    '状态': diff['status']
                })
        
        logger.info(f"✓ 差异报告已导出到: {report_file}")
    except Exception as e:
        logger.error(f"✗ 导出报告失败: {str(e)}")


def sync_pricing(dry_run: bool = False, target_envs: Optional[Set[str]] = None, 
                 report_file: Optional[str] = None, filter_models: Optional[Set[str]] = None):
    """主同步函数
    
    Args:
        dry_run: 是否为干运行模式（不实际更新）
        target_envs: 只同步到指定的环境（环境key集合）
        report_file: 导出差异报告的文件路径
        filter_models: 只同步特定的模型（模型名称集合）
    """
    logger.info("=" * 80)
    if dry_run:
        logger.info("开始分析模型价格差异（干运行模式 - 不会实际更新）")
    else:
        logger.info("开始同步模型价格")
    logger.info("=" * 80)
    
    if target_envs:
        logger.info(f"目标环境: {', '.join(target_envs)}")
    if filter_models:
        logger.info(f"过滤模型: {', '.join(filter_models)}")
    
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
        
        # 如果指定了模型过滤，只处理指定的模型
        if filter_models and model_name not in filter_models:
            continue
        
        if model_name and model_ratio is not None and completion_ratio is not None:
            source_pricing[model_name] = {
                "model_ratio": model_ratio,
                "completion_ratio": completion_ratio,
                "channel_type": model.get("channel_type")
            }
    
    logger.info(f"✓ 源环境共有 {len(source_pricing)} 个模型有价格信息")
    
    # 3. 同步到目标环境
    logger.info(f"\n{'=' * 80}")
    if dry_run:
        logger.info(f"步骤 2: 分析价格差异（干运行模式）")
    else:
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
    
    all_differences = []
    
    for env_key, env_config in ENVIRONMENTS.items():
        if env_config.get("is_source"):
            continue  # 跳过源环境
        
        # 如果指定了目标环境，只处理指定的环境
        if target_envs and env_key not in target_envs:
            continue
        
        summary["total_envs"] += 1
        
        logger.info(f"\n{'-' * 80}")
        logger.info(f"正在{'分析' if dry_run else '同步到'}: {env_config['name']}")
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
        
        # 同步或分析价格
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
            
            # 记录差异
            diff = {
                'env_name': env_config['name'],
                'model_name': model_name,
                'source_model_ratio': pricing['model_ratio'],
                'target_model_ratio': current_model_ratio,
                'model_ratio_diff': pricing['model_ratio'] - (current_model_ratio or 0),
                'source_completion_ratio': pricing['completion_ratio'],
                'target_completion_ratio': current_completion_ratio,
                'completion_ratio_diff': pricing['completion_ratio'] - (current_completion_ratio or 0),
                'status': '待更新' if not dry_run else '差异'
            }
            all_differences.append(diff)
            
            # 显示差异
            logger.info(f"  模型: {model_name}")
            logger.info(f"    输入价格: {current_model_ratio} -> {pricing['model_ratio']} "
                       f"(差异: {diff['model_ratio_diff']:+.6f})")
            logger.info(f"    输出价格: {current_completion_ratio} -> {pricing['completion_ratio']} "
                       f"(差异: {diff['completion_ratio_diff']:+.6f})")
            
            # 如果不是干运行模式，执行更新
            if not dry_run:
                if target_sync.update_model_pricing(
                    model_name,
                    pricing["model_ratio"],
                    pricing["completion_ratio"]
                ):
                    env_updated += 1
                    diff['status'] = '更新成功'
                    logger.info(f"  ✓ 更新成功")
                else:
                    env_failed += 1
                    diff['status'] = '更新失败'
            else:
                env_updated += 1
        
        # 统计
        summary["success_envs"] += 1
        summary["total_models"] += len(target_models_map)
        summary["updated_models"] += env_updated
        summary["failed_models"] += env_failed
        summary["skipped_models"] += env_skipped
        
        logger.info(f"\n{env_config['name']} {'分析' if dry_run else '同步'}完成:")
        logger.info(f"  - 目标环境模型数: {len(target_models_map)}")
        if dry_run:
            logger.info(f"  - 发现差异: {env_updated}")
        else:
            logger.info(f"  - 更新成功: {env_updated}")
            logger.info(f"  - 更新失败: {env_failed}")
        logger.info(f"  - 跳过（无此模型或价格相同）: {env_skipped}")
    
    # 4. 导出报告
    if report_file and all_differences:
        logger.info(f"\n正在导出差异报告...")
        export_diff_report(all_differences, report_file)
    
    # 5. 打印总结
    logger.info(f"\n{'=' * 80}")
    logger.info(f"{'分析' if dry_run else '同步'}完成 - 总结")
    logger.info(f"{'=' * 80}")
    logger.info(f"环境统计:")
    logger.info(f"  - 目标环境总数: {summary['total_envs']}")
    logger.info(f"  - {'分析' if dry_run else '同步'}成功: {summary['success_envs']}")
    logger.info(f"  - {'分析' if dry_run else '同步'}失败: {summary['failed_envs']}")
    logger.info(f"\n模型统计:")
    logger.info(f"  - 目标环境模型总数: {summary['total_models']}")
    if dry_run:
        logger.info(f"  - 发现价格差异: {summary['updated_models']}")
    else:
        logger.info(f"  - 价格更新成功: {summary['updated_models']}")
        logger.info(f"  - 价格更新失败: {summary['failed_models']}")
    logger.info(f"  - 跳过（无此模型或价格相同）: {summary['skipped_models']}")
    logger.info(f"{'=' * 80}\n")
    
    if dry_run and summary['updated_models'] > 0:
        logger.info("提示: 这是干运行模式，没有执行实际更新。")
        logger.info("      如需执行实际更新，请移除 --dry-run 参数。\n")


def main():
    parser = argparse.ArgumentParser(
        description='模型价格同步脚本（增强版）',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 干运行模式（不实际更新）
  %(prog)s --dry-run
  
  # 只同步到指定环境
  %(prog)s --env dev,enetest
  
  # 导出差异报告
  %(prog)s --dry-run --report pricing_diff.csv
  
  # 只同步特定模型
  %(prog)s --models gpt-4o,gpt-4o-mini
  
  # 组合使用
  %(prog)s --env dev --models gpt-4o --dry-run
        """
    )
    
    parser.add_argument(
        '--dry-run',
        action='store_true',
        help='干运行模式：只分析差异，不实际更新'
    )
    
    parser.add_argument(
        '--env',
        type=str,
        help='只同步到指定环境（用逗号分隔），例如: dev,enetest'
    )
    
    parser.add_argument(
        '--report',
        type=str,
        metavar='FILE',
        help='导出价格差异报告到CSV文件'
    )
    
    parser.add_argument(
        '--models',
        type=str,
        help='只同步特定模型（用逗号分隔），例如: gpt-4o,gpt-4o-mini'
    )
    
    parser.add_argument(
        '--list-envs',
        action='store_true',
        help='列出所有可用的环境'
    )
    
    args = parser.parse_args()
    
    # 列出环境
    if args.list_envs:
        print("\n可用环境:")
        print("-" * 60)
        for key, config in ENVIRONMENTS.items():
            if config.get("is_source"):
                print(f"  {key:15} - {config['name']} (源环境)")
            else:
                print(f"  {key:15} - {config['name']}")
        print()
        return
    
    # 解析目标环境
    target_envs = None
    if args.env:
        target_envs = set(env.strip() for env in args.env.split(','))
        invalid_envs = target_envs - set(ENVIRONMENTS.keys())
        if invalid_envs:
            logger.error(f"无效的环境: {', '.join(invalid_envs)}")
            logger.info("使用 --list-envs 查看所有可用环境")
            return
    
    # 解析模型过滤
    filter_models = None
    if args.models:
        filter_models = set(model.strip() for model in args.models.split(','))
        logger.info(f"将只同步以下模型: {', '.join(filter_models)}")
    
    try:
        sync_pricing(
            dry_run=args.dry_run,
            target_envs=target_envs,
            report_file=args.report,
            filter_models=filter_models
        )
    except KeyboardInterrupt:
        logger.info("\n用户中断操作")
    except Exception as e:
        logger.error(f"\n发生未预期的错误: {str(e)}", exc_info=True)


if __name__ == "__main__":
    main()

