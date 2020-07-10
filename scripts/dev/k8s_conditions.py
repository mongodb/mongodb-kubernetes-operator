import time
from typing import Callable, Tuple, List, Optional, Any

from kubernetes.client.rest import ApiException

# time to sleep between retries
SLEEP_TIME = 2
# no timeout (loop forever)
INFINITY = -1


def _current_milliseconds() -> int:
    return int(round(time.time() * 1000))


def wait(
    fn: Callable,
    condition: Callable,
    exceptions_to_ignore: Optional[Tuple[BaseException]] = None,
    codes_to_ignore: Optional[List[int]] = None,
    sleep_time: float = SLEEP_TIME,
    timeout: int = INFINITY,
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
        except exceptions_to_ignore:  # type: ignore
            # The above comment is  due to an issue in mypy with tuple of Exceptions
            pass
        if res is not None and condition(res):
            return True

        time.sleep(sleep_time)

    return False


def call_eventually_succeeds(
    fn: Callable,
    sleep_time: float = SLEEP_TIME,
    timeout: int = INFINITY,
    exceptions_to_ignore: Optional[Tuple[BaseException]] = None,
) -> bool:
    """
    call_eventually_succeeds is similar to wait but in this case we ignore the value returned by the call to fn()

    It periodically calls fn and if the call raises an exception contained in exceptions_to_ignore, it tries
    again after a sleep until it either succeeds or we reach timeout
    """
    start_time = _current_milliseconds()
    end = start_time + (timeout * 1000)

    while _current_milliseconds() < end or timeout <= 0:
        try:
            fn()
            return True
        except exceptions_to_ignore:  # type: ignore
            pass

        time.sleep(sleep_time)

    return False


def _ignore_error_codes(fn: Callable, codes: Optional[List[int]]) -> Any:
    try:
        return fn()
    except ApiException as e:
        if codes is not None and e.status not in codes:
            raise


def ignore_if_already_exists(fn: Callable) -> Any:
    """
    ignore_if_already_exists accepts a function and calls it,
    ignoring an Kubernetes API conflict errors
    """
    return _ignore_error_codes(fn, [409])


def ignore_if_doesnt_exist(fn: Callable) -> Any:
    """
    ignore_if_doesnt_exist accepts a function and calls it,
    ignoring an Kubernetes API not found errors
    """
    return _ignore_error_codes(fn, [404])
