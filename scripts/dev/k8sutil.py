import time

from kubernetes.client.rest import ApiException

# time to sleep between retries
SLEEP_TIME = 2
# no timeout (loop forever)
INFINITY = -1

def _current_milliseconds():
    return int(round(time.time() * 1000))

def wait_for_k8s_api_condition(fn, condition, codes_to_ignore=[],sleep_time=SLEEP_TIME, timeout=INFINITY) -> bool:
    start_time = _current_milliseconds()
    end = start_time + (timeout * 1000)

    while _current_milliseconds() < end or timeout <= 0:
        res =  _ignore_error_codes(fn,codes_to_ignore)
        if condition(res) == True:
            return True

        time.sleep(sleep_time)

    return False


def _ignore_error_codes(fn, codes):
    try:
        return fn()
    except ApiException as e:
        if e.status not in codes:
            raise


def ignore_if_already_exists(fn):
    """
    ignore_if_already_exists accepts a function and calls it,
    ignoring an Kubernetes API conflict errors
    """

    return _ignore_error_codes(fn, [409])


def ignore_if_doesnt_exist(fn):
    """
    ignore_if_doesnt_exist accepts a function and calls it,
    ignoring an Kubernetes API not found errors
    """
    return _ignore_error_codes(fn, [404])

