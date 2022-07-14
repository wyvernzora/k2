set -g fish_user_paths /bin /sbin /usr/bin /usr/sbin /usr/local/bin /usr/local/sbin

if status is-interactive
    # Color scheme from base16-fish
    set -g base16_theme snazzy

    # Command prompt configuration
    set -g tide_prompt_icon_connection '·'
    set -g tide_prompt_color_frame_and_connection 'brblack'
    set -g tide_left_prompt_items pwd context git newline character
    set -g tide_right_prompt_items status cmd_duration jobs time
end
