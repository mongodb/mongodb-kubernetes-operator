import time

from kubernetes.client.rest import ApiException

# time to sleep between retries
SLEEP_TIME = 2
# no timeout (loop forever)
INFINITY = -1


def _current_milliseconds() -> int:
    return int(round(time.time() * 1000))


def wait_for_condition(
    fn,
    condition,
    exceptions_to_ignore=None,
    codes_to_ignore=None,
    sleep_time=SLEEP_TIME,
    timeout=INFINITY,
) -> bool:
    """
    wait_for_condition accepts a function fn and a function condition,
    it periodically calls the function fn and then applies the condition function on the result
    until it returns True or we reach timeout

    exceptions_to_ignore is a tuple of Exceptions to ignore is raised by the call to fn
    If ApiException is not ignored, if raised by the call to fn codes in codes_to_ignore are ignored
    """
    start_time = _current_milliseconds()
    end = start_time + (timeout * 1000)

    while _current_milliseconds() < end or timeout <= 0:
        res = None
        try:
            res = _ignore_error_codes(fn, codes_to_ignore)
        except exceptions_to_ignore:
            pass
        if res is not None and condition(res):
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
