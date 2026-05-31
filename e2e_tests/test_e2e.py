from playwright.sync_api import Page, expect

def test_desktop_layout(page: Page):
    # Set desktop viewport size
    page.set_viewport_size({"width": 1920, "height": 1080})
    page.goto("http://localhost:8080")
    
    # Locate layout columns
    sidebar = page.locator(".sidebar")
    widgets = page.locator(".widgets-sidebar")
    hamburger = page.locator(".navbar-hamburger")
    
    # Assert desktop responsiveness layout
    expect(sidebar).to_be_visible()
    expect(widgets).to_be_visible()
    expect(hamburger).to_be_hidden()

def test_mobile_layout_and_hamburger(page: Page):
    # Set mobile viewport size
    page.set_viewport_size({"width": 375, "height": 667})
    page.goto("http://localhost:8080")
    
    # Locate layout columns
    sidebar = page.locator(".sidebar")
    widgets = page.locator(".widgets-sidebar")
    hamburger = page.locator(".navbar-hamburger")
    
    # Assert mobile responsiveness layout and hamburger menu visibility
    expect(sidebar).to_be_hidden()
    expect(widgets).to_be_hidden()
    expect(hamburger).to_be_visible()
