#if !defined(__STR_H__)
#define __STR_H__

#include "comm.h"
#include <stdbool.h>

/* 字串 */
typedef struct
{
    char *str;      /* 字串值 */
    size_t len;     /* 字串长 */
} str_t;
 
str_t *str_to_lower(str_t *s);
char *char_to_lower(const char *str, char *dstr, int len);
str_t *str_to_upper(str_t *s);
int str_to_hex(const char *str, int len, char *hex);
bool str_isdigit(const char *str);
size_t str_to_num(const char *str);

#endif /*__STR_H__*/
