package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
)

// Courses are the owner's alone: they say where and when the owner studied, and with whom.
func (h *handlers) registerCourses() {
	type courseOutput struct {
		Body *catalog.CourseDetail
	}
	type courseID struct {
		ID int64 `path:"id"`
	}
	type courseCopy struct {
		ID     int64 `path:"id"`
		CopyID int64 `path:"copyId"`
	}

	huma.Register(h.api, h.op("list-courses", http.MethodGet, "/courses", "Every course, latest first"),
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Courses []catalog.Course `json:"courses"`
			}
		}, error) {
			list, err := h.Catalog.Courses(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Courses []catalog.Course `json:"courses"`
				}
			}{}
			out.Body.Courses = list
			return out, nil
		})

	create := h.op("create-course", http.MethodPost, "/courses", "Record a course")
	create.DefaultStatus = http.StatusCreated
	huma.Register(h.api, create, func(ctx context.Context, in *struct{ Body catalog.CourseInput }) (*courseOutput, error) {
		c, err := h.Catalog.CreateCourse(ctx, in.Body)
		if err != nil {
			return nil, h.fail(err)
		}
		return &courseOutput{Body: c}, nil
	})

	huma.Register(h.api, h.op("get-course", http.MethodGet, "/courses/{id}", "A course with its books"),
		func(ctx context.Context, in *courseID) (*courseOutput, error) {
			c, err := h.Catalog.GetCourse(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			return &courseOutput{Body: c}, nil
		})

	huma.Register(h.api, h.op("update-course", http.MethodPut, "/courses/{id}", "Replace a course's details"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.CourseInput
		}) (*courseOutput, error) {
			c, err := h.Catalog.UpdateCourse(ctx, in.ID, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			return &courseOutput{Body: c}, nil
		})

	huma.Register(h.api, h.op("delete-course", http.MethodDelete, "/courses/{id}", "Remove a course; its books stay"),
		func(ctx context.Context, in *courseID) (*struct{}, error) {
			if err := h.Catalog.DeleteCourse(ctx, in.ID); err != nil {
				return nil, h.fail(err)
			}
			return nil, nil
		})

	huma.Register(h.api, h.op("put-course-copy", http.MethodPut, "/courses/{id}/copies/{copyId}", "Put a copy on a course's list, or change whether it was required"),
		func(ctx context.Context, in *struct {
			ID     int64 `path:"id"`
			CopyID int64 `path:"copyId"`
			Body   struct {
				Required bool `json:"required"`
			}
		}) (*courseOutput, error) {
			c, err := h.Catalog.UseInCourse(ctx, in.ID, in.CopyID, in.Body.Required)
			if err != nil {
				return nil, h.fail(err)
			}
			return &courseOutput{Body: c}, nil
		})

	huma.Register(h.api, h.op("delete-course-copy", http.MethodDelete, "/courses/{id}/copies/{copyId}", "Take a copy off a course's list; the copy stays"),
		func(ctx context.Context, in *courseCopy) (*struct{}, error) {
			if err := h.Catalog.LeaveCourse(ctx, in.ID, in.CopyID); err != nil {
				return nil, h.fail(err)
			}
			return nil, nil
		})
}
