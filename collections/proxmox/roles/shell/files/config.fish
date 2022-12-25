set -U fish_user_paths /bin /sbin /usr/bin /usr/sbin /usr/local/bin /usr/local/sbin

# Color scheme from base16-fish
set -U base16_theme snazzy

# Command prompt configuration
set -U tide_prompt_icon_connection '·'
set -U tide_prompt_color_frame_and_connection 'brblack'
set -U tide_left_prompt_items pwd context git newline character
