import pytest
import sys

sys.path.append("..")
from utils.util_log import test_log as log


def ip_check(ip):
    if ip == "localhost":
        return True

    if not isinstance(ip, str):
        log.error("[IP_CHECK] IP(%s) is not a string." % ip)
        return False

    _list = ip.split('.')
    if len(_list) != 4:
        log.error("[IP_CHECK] IP(%s) is wrong, please check manually." % ip)
        return False

    for i in _list:
        if not str(i).isdigit():
            log.error("[IP_CHECK] IP(%s) is wrong, please check manually." % ip)
            return False

    return True


def number_check(num):
    if str(num).isdigit():
        return True

    else:
        log.error("[NUMBER_CHECK] Number(%s) is not a numbers." % num)
        return False


def exist_check(param, _list):
    if param in _list:
        return True

    else:
        log.error("[EXIST_CHECK] Param(%s) is not in (%s)" % (param, _list))
        return False
